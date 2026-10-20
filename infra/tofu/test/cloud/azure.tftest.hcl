# Tests of the module azure with a mocked azurerm provider (brief M7-16 requirements 13 to 16).
# `command = apply` against the mock creates nothing: every resource exists only in the memory
# of `tofu test`. The mocked IDs are valid Azure resource IDs, because the provider parses them.

mock_provider "azurerm" {
  mock_resource "azurerm_resource_group" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/bothy-demo"
    }
  }
  mock_resource "azurerm_subnet" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/bothy-demo/providers/Microsoft.Network/virtualNetworks/bothy-demo/subnets/host"
    }
  }
  mock_resource "azurerm_network_security_group" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/bothy-demo/providers/Microsoft.Network/networkSecurityGroups/bothy-demo"
    }
  }
  mock_resource "azurerm_public_ip" {
    defaults = {
      id         = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/bothy-demo/providers/Microsoft.Network/publicIPAddresses/bothy-demo"
      ip_address = "192.0.2.20"
    }
  }
  mock_resource "azurerm_network_interface" {
    defaults = {
      id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/bothy-demo/providers/Microsoft.Network/networkInterfaces/bothy-demo"
    }
  }
}

variables {
  instance_name         = "demo"
  environment           = "prod"
  bothy_version         = "1.2.3"
  image_registry        = "ghcr.io/alex"
  public_hostname       = "chat.example.com"
  release_repository    = "alex/bothy"
  owner_handle          = "alex"
  owner_name            = "Alex Example"
  admin_ssh_public_keys = ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGli5i6DCRTCblR1XFviXAN6md6ke0167D63NKgFFxHK alex.com"]
  location              = "westeurope"
  vm_size               = "Standard_B2s"
}

run "edge_port_only_public" {
  command = apply

  module {
    source = "../../modules/azure"
  }

  assert {
    condition = alltrue([
      for r in azurerm_network_security_group.host.security_rule : r.destination_port_range == "443" && r.protocol == "Tcp"
      if r.direction == "Inbound" && r.access == "Allow" && contains(["*", "Internet", "0.0.0.0/0"], r.source_address_prefix)
    ])
    error_message = "Only the edge port 443 may be open to the internet."
  }
  assert {
    condition     = length([for r in azurerm_network_security_group.host.security_rule : r if r.direction == "Inbound" && r.access == "Allow"]) == 1
    error_message = "reverse_proxy without ssh_allowed_cidrs allows exactly the edge port."
  }
  assert {
    condition = anytrue([
      for r in azurerm_network_security_group.host.security_rule :
      r.direction == "Inbound" && r.access == "Deny" && r.source_address_prefix == "*" && r.destination_port_range == "*" && r.priority == 4096
    ])
    error_message = "Every other inbound packet is denied, also from inside the virtual network."
  }
  assert {
    condition     = azurerm_subnet_network_security_group_association.host.network_security_group_id == azurerm_network_security_group.host.id
    error_message = "The subnet of the host sits behind the security group."
  }
  assert {
    condition     = azurerm_network_interface_security_group_association.host.network_security_group_id == azurerm_network_security_group.host.id
    error_message = "The interface of the host sits behind the security group."
  }
  assert {
    condition     = azurerm_linux_virtual_machine.host.custom_data == base64encode(module.host.user_data)
    error_message = "The host must get the user data of generic-vm."
  }
  assert {
    condition     = length(azurerm_linux_virtual_machine.host.custom_data) <= 87380
    error_message = "Azure accepts at most 64 KiB of custom data (87380 characters in base64)."
  }
  assert {
    condition     = azurerm_linux_virtual_machine.host.disable_password_authentication
    error_message = "The host accepts SSH keys only."
  }
  assert {
    condition     = output.public_url == "https://chat.example.com"
    error_message = "public_url is https://<public_hostname>."
  }
  assert {
    condition     = output.database_endpoint == "" && output.object_storage_bucket == "" && output.secret_store_ref == ""
    error_message = "azure runs PostgreSQL and the files in the stack and keeps the secrets on the host."
  }
  assert {
    condition     = endswith(output.ssh_endpoint, "@192.0.2.20")
    error_message = "ssh_endpoint names the public address of the host."
  }
  assert {
    condition     = length(azurerm_resource_group.instance) == 1
    error_message = "Without resource_group_name the module creates its resource group."
  }
}

run "no_public_port_without_reverse_proxy" {
  command = apply

  module {
    source = "../../modules/azure"
  }

  variables {
    edge_mode           = "tailscale"
    ssh_allowed_cidrs   = ["198.51.100.0/24"]
    resource_group_name = "bothy-existing"
  }

  assert {
    condition = alltrue([
      for r in azurerm_network_security_group.host.security_rule : r.destination_port_range == "22" && r.source_address_prefix == "198.51.100.0/24"
      if r.direction == "Inbound" && r.access == "Allow"
    ])
    error_message = "SSH is allowed only from ssh_allowed_cidrs, nothing else."
  }
  assert {
    condition     = length(azurerm_resource_group.instance) == 0 && azurerm_linux_virtual_machine.host.resource_group_name == "bothy-existing"
    error_message = "An existing resource group is used as it is."
  }
}

run "ssh_from_anywhere_is_refused" {
  command = plan

  module {
    source = "../../modules/azure"
  }

  variables {
    ssh_allowed_cidrs = ["::/0"]
  }

  expect_failures = [var.ssh_allowed_cidrs]
}

run "managed_database_is_refused" {
  command = plan

  module {
    source = "../../modules/azure"
  }

  variables {
    database_mode = "managed"
  }

  expect_failures = [var.database_mode]
}

run "blob_storage_is_refused" {
  command = plan

  module {
    source = "../../modules/azure"
  }

  variables {
    object_storage_mode = "s3"
  }

  expect_failures = [var.object_storage_mode]
}

run "a_host_without_ssh_key_is_refused" {
  command = plan

  module {
    source = "../../modules/azure"
  }

  variables {
    admin_ssh_public_keys = []
  }

  expect_failures = [var.admin_ssh_public_keys]
}
