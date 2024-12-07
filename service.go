package main

import (
	"log"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type NatService struct {
	watcher       *fsnotify.Watcher
	domainMap     map[string]string
	mux           sync.RWMutex
	latestScript  string
	configPath    []string
	TestMode      bool // testMode Only Generate nat script but not apply
	ConvertMode   bool // convertMode Only Convert iptables rules to nftables rules
	GlobalLocalIP string
}

func NewNatService() *NatService {
	// Create new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Failed to create watcher", "error", err)
	}
	defer watcher.Close()

	return &NatService{
		watcher:      watcher,
		domainMap:    make(map[string]string),
		latestScript: "",
	}

}

// ConvertTask
func (s *NatService) ConvertTask(path string, convertPath string) {
	s.ConvertMode = true
	s.TestMode = true
	s.AddConfig(path)
	var netcells []NatCell
	for _, path := range s.configPath {
		slog.Info("Read config file", "path", path)
		netcells = append(netcells, ReadConfig(path)...)
	}
	slog.Info("Read config file Done", "total", len(netcells))
	script := s.GenerateScript(netcells)
	slog.Info("Convert iptables rules to nftables rules", "path", convertPath)
	// 写入 ConvertPath

	if err := os.WriteFile(convertPath, []byte(script), 0644); err != nil {
		slog.Error("Failed to write script", "error", err)
		return
	}
	slog.Info("Convert iptables rules to nftables rules Done", "path", convertPath)

}

func (s *NatService) AddConfig(path string) *NatService {
	// 检查是文件还是目录
	fi, err := os.Stat(path)
	if err != nil {
		slog.Error("Failed to stat", "path", path, "error", err)
		return s
	}
	// 如果是目录，遍历一层，添加所有文件
	if fi.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			slog.Error("Failed to read directory", "path", path, "error", err)
			return s
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			s.AddSingleConfig(filepath.Join(path, file.Name()))
		}
	} else {
		s.AddSingleConfig(path)
	}
	return s
}

func (s *NatService) AddSingleConfig(path string) {
	if !s.ConvertMode {
		if err := s.watcher.Add(path); err != nil {
			slog.Error("Failed to add watcher", "error", err)
		}
	}
	s.configPath = append(s.configPath, path)
	slog.Info("Added config file", "path", path)
}

func (s *NatService) RefreshDomainMap() {
	s.mux.Lock()
	defer s.mux.Unlock()
	if len(s.domainMap) == 0 {
		return
	}
	var err error
	var refreshSuccessNum int
	for str := range s.domainMap {
		s.domainMap[str], err = getRemoteIP(str)
		if err != nil {
			slog.Error("Failed to resolve domain", "domain", str, "error", err)
			continue
		}
		refreshSuccessNum++
	}

	slog.Info("Refresh domain map Done", "total", len(s.domainMap), "success", refreshSuccessNum)
}

func (s *NatService) CleanMap() {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.domainMap = make(map[string]string)
}

func (s *NatService) Run() {
	s.Sync()

	go func() {
		for {
			s.RefreshDomainMap()
			// Refresh every 1 minutes  |||  todo configurable
			<-time.After(1 * time.Minute)
		}
	}()
	// Watch for changes
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				slog.Info("Config file modified, Reload Config", "path", event.Name)
				s.CleanMap()
				s.Sync()
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v\n", err)
		}
	}
}

func (s *NatService) Sync() {
	var netcells []NatCell
	for _, path := range s.configPath {
		netcells = append(netcells, ReadConfig(path)...)
	}
	script := s.GenerateScript(netcells)
	s.applyScript(script)

}

func (s *NatService) InitEnv() *NatService {
	// Create nftables directory if not exists
	if err := os.MkdirAll(nftablesEtc, 0755); err != nil {
		slog.Error("Failed to create directory", "path", nftablesEtc, "error", err)
		return s
	}

	// Enable IP forwarding
	// if err := os.WriteFile(ipForward, []byte("1"), 0644); err != nil {
	// 	slog.Error("Failed to enable ip_forward", "error", err, "message", "Please execute 'echo 1 > /proc/sys/net/ipv4/ip_forward' manually")
	// 	return s
	// } else {
	// 	slog.Info("Kernel ip_forward config enabled!")
	// }

	return s
}

func (s *NatService) GenerateScript(config []NatCell) string {
	var localIP string
	localIP = s.GlobalLocalIP
	if localIP == "" {
		localIP = os.Getenv("nat_local_ip")
		if localIP == "" {
			var err error
			localIP, err = getLocalIP()
			if err != nil {
				slog.Error("Failed to get local IP", "error", err)
				return ""
			}
		}
	}
	script := scriptPrefix
	for _, entry := range config {
		entry.LocalIP = localIP
		entry.DstIP = s.parseEntryDomain(entry)
		slog.Debug("Generate Entry", "entry", entry)
		script += entry.Build()
	}
	return script
}

func (s *NatService) parseEntryDomain(entry NatCell) string {
	if ip := net.ParseIP(entry.DstDomain); ip != nil {
		return ip.String()
	}
	s.mux.RLock()
	if ip, ok := s.domainMap[entry.DstDomain]; ok && ip != "" {
		s.mux.RUnlock()
		return ip
	}
	s.mux.RUnlock()

	ip, err := getRemoteIP(entry.DstDomain)
	if err != nil {
		slog.Error("Failed to resolve domain", "domain", entry.DstDomain, "error", err)
		return ""
	}
	s.mux.Lock()
	s.domainMap[entry.DstDomain] = ip
	s.mux.Unlock()
	return ip
}

func (s *NatService) applyScript(script string) {
	slog.Info("nftables script", "script", script)
	s.latestScript = script

	scriptPath := filepath.Join(nftablesEtc, "nat-diy.nft")
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		slog.Error("Failed to write script", "error", err)
		return
	}

	if s.TestMode {
		return
	}

	cmd := exec.Command("/usr/sbin/nft", "-f", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("Failed to execute nft command", "error", err, "output", string(output))
	} else {
		slog.Info("Executed nft command", "path", scriptPath, "result", "success")
	}
}
