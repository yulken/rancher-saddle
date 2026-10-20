package generator

import (
	"os"
	"path/filepath"
	"text/template"

	"github.com/Felipalds/rancher-tui/internal/model"
)

const ansibleTemplate = `
- name: Initialize RKE2 on First Node
  hosts: init
  become: yes
  vars:
    rke2_version: "{{.RKE2Version}}"
  
  tasks:
    - name: Wait for cloud-init
      command: cloud-init status --wait
      changed_when: false

    - name: Install RKE2 server (Init)
      shell: curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION="{{"{{"}} rke2_version {{"}}"}}" sh -
      args:
        creates: /usr/local/bin/rke2

    - name: Enable and start RKE2 Server
      systemd:
        name: rke2-server
        state: started
        enabled: yes

    - name: Wait for Node Token
      wait_for:
        path: /var/lib/rancher/rke2/server/node-token

    - name: Fetch Node Token
      slurp:
        src: /var/lib/rancher/rke2/server/node-token
      register: rke2_token_base64

    - name: Get private IP address
      shell: hostname -I | awk '{print $1}'
      register: private_ip
      changed_when: false

    - name: Set Token Fact
      set_fact:
        rke2_token: "{{"{{"}} rke2_token_base64['content'] | b64decode | trim {{"}}"}}"
        rke2_url: "https://{{"{{"}} private_ip.stdout | trim {{"}}"}}:9345"

    - name: Wait for RKE2 server to be listening on port 9345
      wait_for:
        port: 9345
        host: 127.0.0.1
        timeout: 300
        delay: 5

- name: Join RKE2 Nodes
  hosts: join
  become: yes
  vars:
    rke2_version: "{{.RKE2Version}}"
    token: "{{"{{"}} hostvars[groups['init'][0]]['rke2_token'] {{"}}"}}"
    server_url: "{{"{{"}} hostvars[groups['init'][0]]['rke2_url'] {{"}}"}}"

  tasks:
    - name: Wait for cloud-init
      command: cloud-init status --wait
      changed_when: false

    - name: Install RKE2 binaries (Join)
      shell: curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION="{{"{{"}} rke2_version {{"}}"}}" sh -
      args:
        creates: /usr/local/bin/rke2

    - name: Create RKE2 config directory
      file:
        path: /etc/rancher/rke2
        state: directory
        mode: '0755'

    - name: Create RKE2 config file for joining
      copy:
        dest: /etc/rancher/rke2/config.yaml
        content: |
          server: {{"{{"}} server_url {{"}}"}}
          token: {{"{{"}} token {{"}}"}}
        mode: '0600'

    - name: Enable and start RKE2 Server
      systemd:
        name: rke2-server
        state: started
        enabled: yes

    - name: Wait for node to join cluster
      command: /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml get nodes
      register: nodes_check
      until: nodes_check.rc == 0
      retries: 30
      delay: 10
      changed_when: false

- name: Deploy Rancher Management Server
  hosts: init
  become: yes
  vars:
    rancher_version: "{{.RancherVersion}}"
    
  tasks:
    - name: Install Helm
      shell: curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 && chmod 700 get_helm.sh && ./get_helm.sh
      args:
        creates: /usr/local/bin/helm

    - name: Set up kubectl and kubeconfig for convenience
      lineinfile:
        path: /root/.bashrc
        line: "{{"{{"}} item {{"}}"}}"
      loop:
        - 'export PATH=$PATH:/var/lib/rancher/rke2/bin'
        - 'export KUBECONFIG=/etc/rancher/rke2/rke2.yaml'

    - name: Ensure .kube directory exists
      file:
        path: /root/.kube
        state: directory
        mode: '0755'

    - name: Make Kubeconfig available
      copy:
        src: /etc/rancher/rke2/rke2.yaml
        dest: /root/.kube/config
        remote_src: yes
        mode: '0600'

    - name: Wait for RKE2 to be ready
      command: /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml get nodes
      register: result
      until: result.rc == 0
      retries: 30
      delay: 10
      changed_when: false

    - name: Install Cert-Manager
      command: /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.1/cert-manager.yaml

    - name: Wait for Cert-Manager Deployments
      command: /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml wait --for=condition=Available deployment --all -n cert-manager --timeout=300s
      changed_when: false

    - name: Wait for Cert-Manager Webhook Service
      command: /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml get endpoints cert-manager-webhook -n cert-manager -o jsonpath='{.subsets[*].addresses[*].ip}'
      register: webhook_endpoints
      until: webhook_endpoints.stdout != ""
      retries: 30
      delay: 10
      changed_when: false

    - name: Wait Additional Time for Webhook Registration
      pause:
        seconds: 30

    - name: Add Rancher Helm Repos
      command: /usr/local/bin/helm repo add {{"{{"}} item.name {{"}}"}} {{"{{"}} item.url {{"}}"}}
      loop:
        - { name: 'rancher-latest', url: 'https://releases.rancher.com/server-charts/latest' }
        - { name: 'jetstack', url: 'https://charts.jetstack.io' }

    - name: Update Helm Repos
      command: /usr/local/bin/helm repo update

    - name: Create Cattle System Namespace
      command: /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml create namespace cattle-system
      ignore_errors: yes

    - name: Install or Upgrade Rancher
      command: >
        /usr/local/bin/helm upgrade --install rancher rancher-latest/rancher
        --namespace cattle-system
        --set hostname={{"{{"}} rancher_hostname {{"}}"}}
        --set bootstrapPassword=admin
        --set replicas=1
        --version {{"{{"}} rancher_version {{"}}"}}
        --kubeconfig /etc/rancher/rke2/rke2.yaml
        --create-namespace
`

// GenerateAnsible creates the site.yml file based on the provided configuration.
func GenerateAnsible(config *model.Config, outputDir string) error {
	path := filepath.Join(outputDir, "site.yml")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("ansible").Parse(ansibleTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, config); err != nil {
		return err
	}

	return nil
}
