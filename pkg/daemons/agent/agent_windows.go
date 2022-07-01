//go:build windows
// +build windows

package agent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
)

var NetworkName = "flannel.4096"

func checkRuntimeEndpoint(cfg *config.Agent, argsMap map[string]string) {
	if strings.HasPrefix(cfg.RuntimeSocket, windowsPrefix) {
		argsMap["container-runtime-endpoint"] = cfg.RuntimeSocket
	} else {
		argsMap["container-runtime-endpoint"] = windowsPrefix + cfg.RuntimeSocket
	}
}

func kubeProxyArgs(cfg *config.Agent) map[string]string {
	bindAddress := "127.0.0.1"
	_, IPv6only, _ := util.GetFirstString([]string{cfg.NodeIP})
	if IPv6only {
		bindAddress = "::1"
	}
	argsMap := map[string]string{
		"proxy-mode":           "kernelspace",
		"healthz-bind-address": bindAddress,
		"kubeconfig":           cfg.KubeConfigKubeProxy,
		"cluster-cidr":         util.JoinIPNets(cfg.ClusterCIDRs),
	}
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
	}

	if sourceVip := waitForSourceVip(NetworkName); sourceVip != "" {
		argsMap["source-vip"] = sourceVip
	}

	return argsMap
}

func kubeletArgs(cfg *config.Agent) map[string]string {
	bindAddress := "127.0.0.1"
	_, IPv6only, _ := util.GetFirstString([]string{cfg.NodeIP})
	if IPv6only {
		bindAddress = "::1"
	}
	argsMap := map[string]string{
		"healthz-bind-address":         bindAddress,
		"read-only-port":               "0",
		"cluster-domain":               cfg.ClusterDomain,
		"kubeconfig":                   cfg.KubeConfigKubelet,
		"eviction-hard":                "imagefs.available<5%,nodefs.available<5%",
		"eviction-minimum-reclaim":     "imagefs.available=10%,nodefs.available=10%",
		"fail-swap-on":                 "false",
		"authentication-token-webhook": "true",
		"anonymous-auth":               "false",
		"authorization-mode":           modes.ModeWebhook,
	}
	if cfg.PodManifests != "" && argsMap["pod-manifest-path"] == "" {
		argsMap["pod-manifest-path"] = cfg.PodManifests
	}
	if err := os.MkdirAll(argsMap["pod-manifest-path"], 0755); err != nil {
		logrus.Errorf("Failed to mkdir %s: %v", argsMap["pod-manifest-path"], err)
	}
	if cfg.RootDir != "" {
		argsMap["root-dir"] = cfg.RootDir
		argsMap["cert-dir"] = filepath.Join(cfg.RootDir, "pki")
	}
	if len(cfg.ClusterDNS) > 0 {
		argsMap["cluster-dns"] = util.JoinIPs(cfg.ClusterDNSs)
	}
	if cfg.ResolvConf != "" {
		argsMap["resolv-conf"] = cfg.ResolvConf
	}
	if cfg.RuntimeSocket != "" {
		//argsMap["containerd"] = cfg.RuntimeSocket
		argsMap["serialize-image-pulls"] = "false"
		checkRuntimeEndpoint(cfg, argsMap)
	}
	if cfg.PauseImage != "" {
		argsMap["pod-infra-container-image"] = cfg.PauseImage
	}
	if cfg.ListenAddress != "" {
		argsMap["address"] = cfg.ListenAddress
	}
	if cfg.ClientCA != "" {
		argsMap["anonymous-auth"] = "false"
		argsMap["client-ca-file"] = cfg.ClientCA
	}
	if cfg.ServingKubeletCert != "" && cfg.ServingKubeletKey != "" {
		argsMap["tls-cert-file"] = cfg.ServingKubeletCert
		argsMap["tls-private-key-file"] = cfg.ServingKubeletKey
	}
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
	}
	defaultIP, err := net.ChooseHostInterface()
	if err != nil || defaultIP.String() != cfg.NodeIP {
		argsMap["node-ip"] = cfg.NodeIP
	}

	argsMap["node-labels"] = strings.Join(cfg.NodeLabels, ",")
	if len(cfg.NodeTaints) > 0 {
		argsMap["register-with-taints"] = strings.Join(cfg.NodeTaints, ",")
	}

	if !cfg.DisableCCM {
		argsMap["cloud-provider"] = "external"
	}

	if ImageCredProvAvailable(cfg) {
		logrus.Infof("Kubelet image credential provider bin dir and configuration file found.")
		argsMap["feature-gates"] = util.AddFeatureGate(argsMap["feature-gates"], "KubeletCredentialProviders=true")
		argsMap["image-credential-provider-bin-dir"] = cfg.ImageCredProvBinDir
		argsMap["image-credential-provider-config"] = cfg.ImageCredProvConfig
	}

	if cfg.ProtectKernelDefaults {
		argsMap["protect-kernel-defaults"] = "true"
	}
	return argsMap
}

func waitForSourceVip(networkName string) string {
	for range time.Tick(time.Second * 5) {
		network, err := hcsshim.GetHNSNetworkByName(networkName)
		if err != nil {
			logrus.WithError(err).Warning("can't find HNS network, retrying", networkName)
			continue
		}
		if network.ManagementIP == "" {
			continue
		}

		script := `function GetSourceVip {
			param(
					$NetworkName
				)
			$hnsNetwork = Get-HnsNetwork | ? Name -EQ $NetworkName.ToLower()
    		$subnet = $hnsNetwork.Subnets[0].AddressPrefix
			$ipamConfig = @"
        {"cniVersion": "0.2.0", "name": "vxlan0", "ipam":{"type":"host-local","ranges":[[{"subnet":"$subnet"}]],"dataDir":"/var/lib/cni/networks"}}
"@
			$ipamConfig | Out-File "C:\k\sourceVipRequest.json"
			$env:CNI_COMMAND="ADD"
			$env:CNI_CONTAINERID="dummy"
			$env:CNI_NETNS="dummy"
			$env:CNI_IFNAME="dummy"
			$env:CNI_PATH="dummy"

			If(!(Test-Path c:\k\sourceVip.json)){
				Get-Content c:\k\sourceVipRequest.json | host-local.exe | Out-File c:\k\sourceVip.json
			}

			$sourceVipJSON = Get-Content c:\k\sourceVip.json | ConvertFrom-Json
    		$sourceVip = $sourceVipJSON.ip4.ip.Split("/")[0]

			Remove-Item env:CNI_COMMAND
			Remove-Item env:CNI_CONTAINERID
			Remove-Item env:CNI_NETNS
			Remove-Item env:CNI_IFNAME
			Remove-Item env:CNI_PATH

			return $sourceVip
			}
		`

		sourceVip := executePowershell(script, `GetSourceVip -NetworkName %s`, networkName)

		sourceVip = strings.TrimRight(sourceVip, "\r\n")

		return sourceVip
	}
	return ""
}

func executePowershell(script string, command string, args ...interface{}) (outputJson string) {
	powershellCommand := script
	powershellCommand += fmt.Sprintf(command, args...)

	cmd := exec.Command("powershell.exe", powershellCommand)
	var out bytes.Buffer
	cmd.Stdout = &out
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()

	if err != nil {
		return ""
	}

	return out.String()
}
