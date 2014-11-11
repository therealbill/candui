package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// SentinelPodConfig is a struct carrying information about a Pod's config as
// pulled from the sentinel config file.
type SentinelPodConfig struct {
	IP        string
	Port      int
	Quorum    int
	Name      string
	AuthToken string
	Sentinels map[string]string
}

// SentinelConfig is a struct holding information about the sentinel we are
// running on.
type SentinelConfig struct {
	Name              string
	Host              string
	Port              int
	ManagedPodConfigs map[string]SentinelPodConfig
	Dir               string
}

var sconfig SentinelConfig

// syncableDirectives is the list of directives to sync
// ideally this should also be controllable per invocation
var syncableDirectives []string

// extractSentinelDirective parses the sentinel directives from the
// sentinel config file
func extractSentinelDirective(entries []string) error {
	switch entries[0] {
	case "monitor":
		pname := entries[1]
		port, _ := strconv.Atoi(entries[3])
		quorum, _ := strconv.Atoi(entries[4])
		spc := SentinelPodConfig{Name: pname, IP: entries[2], Port: port, Quorum: quorum}
		spc.Sentinels = make(map[string]string)
		addr := fmt.Sprintf("%s:%d", entries[2], port)
		_, exists := sconfig.ManagedPodConfigs[addr]
		if !exists {
			sconfig.ManagedPodConfigs[entries[1]] = spc
		}
		return nil

	case "auth-pass":
		pname := entries[1]
		pc := sconfig.ManagedPodConfigs[pname]
		pc.AuthToken = entries[2]
		sconfig.ManagedPodConfigs[pname] = pc
		return nil

	case "config-epoch", "leader-epoch", "current-epoch", "down-after-milliseconds", "known-sentinel", "known-slave":
		// We don't use these keys
		return nil

	default:
		err := fmt.Errorf("Unhandled sentinel directive: %+v", entries)
		logger.Warning(err.Error())
		return nil
	}
}

// LoadSentinelConfigFile loads the local config file pulled from the
// environment variable "CONFIGSYNC_SENTINELCONFIGFILE"
func LoadSentinelConfigFile() error {
	file, err := os.Open(config.SentinelConfigFile)
	defer file.Close()
	if err != nil {
		logger.Crit(err.Error())
		os.Exit(1)
	}
	bf := bufio.NewReader(file)
	for {
		rawline, err := bf.ReadString('\n')
		if err == nil || err == io.EOF {
			line := strings.TrimSpace(rawline)
			// ignore comments
			if strings.Contains(line, "#") {
				continue
			}
			entries := strings.Split(line, " ")
			//Most values are key/value pairs
			switch entries[0] {
			case "sentinel": // Have a sentinel directive
				err := extractSentinelDirective(entries[1:])
				if err != nil {
					// TODO: Fix this to return a different error if we can't
					// connect to the sentinel
					logger.Warning("Misshapen sentinel directive: " + line + " err: " + err.Error())
				}
			case "port":
				iport, _ := strconv.Atoi(entries[1])
				sconfig.Port = iport
				//logger.Warning("Local sentinel is bound to port %d", sconfig.Port)
			case "dir":
				sconfig.Dir = entries[1]
			case "bind":
				sconfig.Host = entries[1]
			case "":
				if err == io.EOF {
					return nil
				}
			default:
				logger.Warning("UNhandled Sentinel Directive: %s" + line)
			}
		} else {
			logger.Warning("=============== LOAD FILE ERROR ===============")
			logger.Crit(err.Error())
			os.Exit(1)
		}
	}
}
