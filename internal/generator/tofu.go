package generator

import (
	"os"
	"path/filepath"
	"text/template"

	"github.com/Felipalds/rancher-tui/internal/model"
)

const tofuTemplate = `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "{{.AWSRegion}}"
  # Credentials come from the process environment (see aws.EnvFor). They are
  # intentionally not rendered here.
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_instance" "rancher_node" {
  count         = {{.InstanceCount}}
  ami           = "{{.AMI}}"
  instance_type = "t3.xlarge"

  subnet_id                   = "{{.SubnetID}}"
  vpc_security_group_ids      = ["{{.SecurityGroupID}}"]
  key_name                    = "{{.SSHKeyName}}"
  associate_public_ip_address = true

  root_block_device {
    volume_size           = {{.RootVolumeSize}}
    volume_type           = "gp3"
    delete_on_termination = true
  }

  tags = {
    Name = "{{.NodePrefix}}-${count.index}"
  }
}

output "instance_ips" {
  value = aws_instance.rancher_node[*].public_ip
}

output "instance_dns_names" {
  value = aws_instance.rancher_node[*].public_dns
}
`

// GenerateTofu creates the main.tf file based on the provided configuration.
func GenerateTofu(config *model.Config, outputDir string) error {
	path := filepath.Join(outputDir, "main.tf")

	// Create the directory if it doesn't exist (though usually caller handles this)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("tofu").Parse(tofuTemplate)
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
