package main

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"os"
	"text/template"
)

var values = struct {
	Region             string
	ETCDEndpoints      string
	PodCIDR            string
	ServiceCIDR        string
	K8sVer             string
	HyperkubeImageRepo string
	K8sNetworkPlugin   string
	ContainerRuntime   string
	SecureAPIServers   string
	DNSServiceIP       string
	ControllerIP       string
	TLSConfig          struct {
		CACert        string
		APIServerCert string
		APIServerKey  string
		WorkerCert    string
		WorkerKey     string
	}
}{
	Region:             "eu-central-1",
	ETCDEndpoints:      "http://172.17.4.51:2379",
	PodCIDR:            "10.2.0.0/16",
	ServiceCIDR:        "10.3.0.0/24",
	K8sVer:             "v1.4.0",
	HyperkubeImageRepo: "gcr.io/google_containers/hyperkube-amd64",
	// K8sVer:             "v1.4.3_coreos.0",
	// HyperkubeImageRepo: "quay.io/coreos/hyperkube",
	K8sNetworkPlugin: "cni",
	ContainerRuntime: "docker",
	SecureAPIServers: "https://172.17.4.101",
	DNSServiceIP:     "10.3.0.10",
	ControllerIP:     "172.17.4.101",
	TLSConfig: struct {
		CACert        string
		APIServerCert string
		APIServerKey  string
		WorkerCert    string
		WorkerKey     string
	}{
		CACert:        fileContent("credentials/ca.pem"),
		APIServerCert: fileContent("credentials/apiserver.pem"),
		APIServerKey:  fileContent("credentials/apiserver-key.pem"),
		WorkerCert:    fileContent("credentials/worker.pem"),
		WorkerKey:     fileContent("credentials/worker-key.pem"),
	},
}

func fileContent(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(data)
}

func main() {
	userdata := map[string]string{
		"cloud-config-controller": "userdata/cloud-config-controller",
		"cloud-config-worker":     "userdata/cloud-config-worker",
	}

	for in, out := range userdata {
		err := genUserData(in, out)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func genUserData(in, out string) error {
	err := os.MkdirAll("userdata", 0755)
	if err != nil {
		return err
	}

	tmpl, err := template.ParseFiles(in)
	if err != nil {
		return err
	}

	fd, err := os.Create(out)
	if err != nil {
		return err
	}

	return tmpl.Execute(fd, values)
}
