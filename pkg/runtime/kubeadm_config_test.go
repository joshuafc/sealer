// Copyright © 2021 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/sealerio/sealer/utils"

	"github.com/sealerio/sealer/utils/yaml"

	"github.com/sealerio/sealer/logger"
)

const (
	testKubeadmConfigYaml = `
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
# advertiseAddress: 192.168.2.110
  bindPort: 6443
#nodeRegistration:
#  criSocket: /var/run/dockershim.sock

---
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
#kubernetesVersion: v1.19.8
#controlPlaneEndpoint: "apiserver.cluster.local:6443"
imageRepository: sea.hub:5000/library
networking:
  # dnsDomain: cluster.local
  podSubnet: 100.64.0.0/10
  serviceSubnet: 10.96.0.0/22
apiServer:
  certSANs:
  - 127.0.0.1
  - apiserver.cluster.local
  - 192.168.2.110
  - aliyun-inc.com
  - 10.0.0.2
  - 10.103.97.2
  extraArgs:
    etcd-servers: https://192.168.2.110:2379
    feature-gates: TTLAfterFinished=true,EphemeralContainers=true
    audit-policy-file: "/etc/kubernetes/audit-policy.yml"
    audit-log-path: "/var/log/kubernetes/audit.log"
    audit-log-format: json
    audit-log-maxbackup: '"10"'
    audit-log-maxsize: '"100"'
    audit-log-maxage: '"7"'
    enable-aggregator-routing: '"true"'
  extraVolumes:
  - name: "audit"
    hostPath: "/etc/kubernetes"
    mountPath: "/etc/kubernetes"
    pathType: DirectoryOrCreate
  - name: "audit-log"
    hostPath: "/var/log/kubernetes"
    mountPath: "/var/log/kubernetes"
    pathType: DirectoryOrCreate
  - name: localtime
    hostPath: /etc/localtime
    mountPath: /etc/localtime
    readOnly: true
    pathType: File
controllerManager:
  extraArgs:
    feature-gates: TTLAfterFinished=true,EphemeralContainers=true
    experimental-cluster-signing-duration: 876000h
  extraVolumes:
  - hostPath: /etc/localtime
    mountPath: /etc/localtime
    name: localtime
    readOnly: true
    pathType: File
scheduler:
  extraArgs:
    feature-gates: TTLAfterFinished=true,EphemeralContainers=true
  extraVolumes:
  - hostPath: /etc/localtime
    mountPath: /etc/localtime
    name: localtime
    readOnly: true
    pathType: File
etcd:
  local:
    extraArgs:
      listen-metrics-urls: http://0.0.0.0:2381

---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: "ipvs"
ipvs:
  excludeCIDRs:
  - "10.103.97.2/32"

---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 2m0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupDriver:
cgroupsPerQOS: true
clusterDomain: cluster.local
configMapAndSecretChangeDetectionStrategy: Watch
containerLogMaxFiles: 5
containerLogMaxSize: 10Mi
contentType: application/vnd.kubernetes.protobuf
cpuCFSQuota: true
cpuCFSQuotaPeriod: 100ms
cpuManagerPolicy: none
cpuManagerReconcilePeriod: 10s
enableControllerAttachDetach: true
enableDebuggingHandlers: true
enforceNodeAllocatable:
- pods
eventBurst: 10
eventRecordQPS: 5
evictionHard:
  imagefs.available: 15%
  memory.available: 100Mi
  nodefs.available: 10%
  nodefs.inodesFree: 5%
evictionPressureTransitionPeriod: 5m0s
failSwapOn: true
fileCheckFrequency: 20s
hairpinMode: promiscuous-bridge
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 20s
imageGCHighThresholdPercent: 85
imageGCLowThresholdPercent: 80
imageMinimumGCAge: 2m0s
iptablesDropBit: 15
iptablesMasqueradeBit: 14
kubeAPIBurst: 10
kubeAPIQPS: 5
makeIPTablesUtilChains: true
maxOpenFiles: 1000000
maxPods: 110
nodeLeaseDurationSeconds: 40
nodeStatusReportFrequency: 10s
nodeStatusUpdateFrequency: 10s
oomScoreAdj: -999
podPidsLimit: -1
port: 10250
registryBurst: 10
registryPullQPS: 5
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 4h0m0s
syncFrequency: 1m0s
volumeStatsAggPeriod: 1m0s
`
	testClusterfile = `apiVersion: sealer.cloud/v2
kind: KubeadmConfig
metadata:
  name: default-kubernetes-config
spec:
  localAPIEndpoint:
    advertiseAddress: 192.168.2.110
    bindPort: 6443
  nodeRegistration:
    criSocket: /var/run/dockershim.sock
  kubernetesVersion: v1.19.8
  controlPlaneEndpoint: "apiserver.cluster.local:6443"
  imageRepository: sea.hub:5000/library
  networking:
    podSubnet: 100.64.0.0/10
    serviceSubnet: 10.96.0.0/22
  apiServer:
    certSANs:
      - sealer.cloud
      - 127.0.0.1
      - Partial.custom.config
  clusterDomain: cluster.local
  nodeLeaseDurationSeconds: 99
  nodeStatusReportFrequency: 99s
  nodeStatusUpdateFrequency: 99s
---
apiVersion: sealer.cloud/v2
kind: Cluster
metadata:
  name: default-kubernetes-cluster
spec:
  image: kubernetes:v1.19.8
---
apiVersion: sealer.cloud/v2
kind: Infra
metadata:
  name: alicloud
spec:
  provider: ALI_CLOUD
  ssh:
    passwd: xxx
    port: 2222
  hosts:
    - count: 3
      role: [ master ]
      cpu: 4
      memory: 4
      systemDisk: 100
      dataDisk: [ 100,200 ]
    - count: 3
      role: [ node ]
      cpu: 4
      memory: 4
      systemDisk: 100
      dataDisk: [ 100, 200 ]
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 2m0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupsPerQOS: true
clusterDomain: cluster.local
configMapAndSecretChangeDetectionStrategy: Watch
containerLogMaxFiles: 5
containerLogMaxSize: 10Mi
contentType: application/vnd.kubernetes.protobuf
cpuCFSQuota: true
cpuCFSQuotaPeriod: 100ms
cpuManagerPolicy: none
cpuManagerReconcilePeriod: 10s
enableControllerAttachDetach: true
enableDebuggingHandlers: true
enforceNodeAllocatable:
  - pods
eventBurst: 10
eventRecordQPS: 5
evictionHard:
  imagefs.available: 15%
  memory.available: 100Mi
  nodefs.available: 10%
  nodefs.inodesFree: 5%
evictionPressureTransitionPeriod: 5m0s
failSwapOn: true
fileCheckFrequency: 20s
hairpinMode: promiscuous-bridge
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 20s
imageGCHighThresholdPercent: 85
imageGCLowThresholdPercent: 80
imageMinimumGCAge: 2m0s
iptablesDropBit: 15
iptablesMasqueradeBit: 14
kubeAPIBurst: 10
kubeAPIQPS: 5
makeIPTablesUtilChains: true
maxOpenFiles: 1000000
maxPods: 110
nodeLeaseDurationSeconds: 40
nodeStatusReportFrequency: 10s
nodeStatusUpdateFrequency: 10s
oomScoreAdj: -999
podPidsLimit: -1
port: 10250
registryBurst: 10
registryPullQPS: 5
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 4h0m0s
syncFrequency: 1m0s
volumeStatsAggPeriod: 1m0s
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
networking:
  podSubnet: 100.64.0.0/10
  serviceSubnet: 10.96.0.0/22
apiServer:
  certSANs:
    - default.raw.config
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: 127.0.0.1 
  bindPort: 6443
nodeRegistration:
  criSocket: /var/run/dockershim.sock`
)

func TestKubeadmConfig_LoadFromClusterfile(t *testing.T) {
	type fields struct {
		KubeConfig *KubeadmConfig
	}
	type args struct {
		kubeadmconfig []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "test kubeadm config from Clusterfile",
			fields: fields{&KubeadmConfig{}},
			args: args{
				[]byte(testClusterfile),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := tt.fields.KubeConfig
			testfile := "test-Clusterfile"
			err := ioutil.WriteFile(testfile, tt.args.kubeadmconfig, 0644)
			if err != nil {
				t.Errorf("WriteFile %s error = %v, wantErr %v", testfile, err, tt.wantErr)
			}
			defer func() {
				err = os.Remove(testfile)
				if err != nil {
					t.Errorf("Remove %s error = %v, wantErr %v", testfile, err, tt.wantErr)
				}
			}()
			KubeadmConfig, err := LoadKubeadmConfigs(testfile, utils.DecodeCRDFromFile)
			if err != nil {
				t.Errorf("err: %v", err)
				return
			}
			if err := k.LoadFromClusterfile(KubeadmConfig); (err != nil) != tt.wantErr {
				t.Errorf("LoadFromClusterfile() error = %v, wantErr %v", err, tt.wantErr)
			}
			logger.Info("k.InitConfiguration.Kind", k.InitConfiguration.Kind)
			out, err := yaml.MarshalWithDelimiter(k.InitConfiguration, k.ClusterConfiguration,
				k.JoinConfiguration, k.KubeletConfiguration, k.KubeProxyConfiguration)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalConfigsToYaml() error = %v, wantErr %v", err, tt.wantErr)
			}
			fmt.Println(string(out))
		})
	}
}

func TestKubeadmConfig_Merge(t *testing.T) {
	type fields struct {
		kubeadmConfig *KubeadmConfig
	}
	type args struct {
		defaultKubeadmConfig []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name:   "test kubeadm config merge",
			fields: fields{&KubeadmConfig{}},
			args: args{
				[]byte(testKubeadmConfigYaml),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := tt.fields.kubeadmConfig
			/*			err := k.LoadFromClusterfile("test-kubeConfig.yml")
						if (err != nil) != tt.wantErr {
							t.Errorf("LoadFromClusterfile() error = %v, wantErr %v", err, tt.wantErr)
							return
						}*/
			testfile := "test-kubeadm.yml"
			err := ioutil.WriteFile(testfile, tt.args.defaultKubeadmConfig, 0644)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteFile %s error = %v, wantErr %v", testfile, err, tt.wantErr)
				return
			}
			defer func() {
				err = os.Remove(testfile)
				if err != nil {
					t.Errorf("remove file %s error = %v, wantErr %v", testfile, err, tt.wantErr)
					return
				}
			}()
			err = k.Merge(testfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("Merge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
