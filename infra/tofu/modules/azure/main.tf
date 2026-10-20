# The chat server on Azure (brief M7-16, ADR 0012, infra/tofu/README.md "Structure" item 1): a
# resource group (or an existing one), a virtual network with one subnet, a network security group that
# admits only the edge port from the internet and denies every other inbound packet, a static
# public address and one Linux VM whose custom data comes from generic-vm (Docker, the verified
# release, the setup wizard). PostgreSQL and the files stay in the Compose stack: install.sh has
# no option for an external database, and Azure Blob Storage has no S3 API (docs/spec/16-betrieb.md
# 3 rule 4), so the files use the volume or SeaweedFS (docs/DECISIONS.md D-M7-16-4). The off-site
# target is an S3 bucket elsewhere, passed in backup_offsite, because bothy-backupd writes to S3
# only (D-M7-16-5).

locals {
  name = "bothy-${var.instance_name}"
  tags = merge(var.tags, {
    "bothy-instance"    = var.instance_name
    "bothy-environment" = var.environment
    "managed-by"        = "opentofu"
  })

  resource_group_name = var.resource_group_name == "" ? azurerm_resource_group.instance[0].name : var.resource_group_name

  # The inbound allow rules: the edge port for everyone with reverse_proxy, SSH only from the
  # networks of the operator; a final rule denies everything else, also the traffic from inside
  # the virtual network and from the load balancer that the default rules of Azure allow.
  allow = concat(
    var.edge_mode == "reverse_proxy" ? [{ name = "edge-https", port = "443", source = "Internet" }] : [],
    [for i, c in var.ssh_allowed_cidrs : { name = "ssh-${i}", port = "22", source = c }],
  )
}

module "host" {
  source = "../generic-vm"

  instance_name         = var.instance_name
  environment           = var.environment
  bothy_version         = var.bothy_version
  image_registry        = var.image_registry
  public_hostname       = var.public_hostname
  edge_mode             = var.edge_mode
  database_mode         = var.database_mode
  object_storage_mode   = var.object_storage_mode
  backup_offsite        = var.backup_offsite
  telemetry_exporters   = var.telemetry_exporters
  admin_ssh_public_keys = var.admin_ssh_public_keys
  tags                  = var.tags

  release_repository = var.release_repository
  release_base_url   = var.release_base_url
  release_public_key = var.release_public_key
  owner_handle       = var.owner_handle
  owner_name         = var.owner_name
  virus_scan         = var.virus_scan

  # cloud-init delivery: the address is only the target of the output ssh_endpoint.
  ssh_host = azurerm_public_ip.host.ip_address
}

resource "azurerm_resource_group" "instance" {
  count = var.resource_group_name == "" ? 1 : 0

  name     = local.name
  location = var.location
  tags     = local.tags
}

# ---- Network ---------------------------------------------------------------------------------

resource "azurerm_virtual_network" "instance" {
  name                = local.name
  location            = var.location
  resource_group_name = local.resource_group_name
  address_space       = ["10.42.0.0/16"]
  tags                = local.tags
}

resource "azurerm_subnet" "host" {
  name                 = "host"
  resource_group_name  = local.resource_group_name
  virtual_network_name = azurerm_virtual_network.instance.name
  address_prefixes     = ["10.42.1.0/24"]
}

resource "azurerm_network_security_group" "host" {
  name                = local.name
  location            = var.location
  resource_group_name = local.resource_group_name
  tags                = local.tags

  dynamic "security_rule" {
    for_each = { for i, r in local.allow : r.name => merge(r, { priority = 100 + i }) }
    content {
      name                       = security_rule.key
      priority                   = security_rule.value.priority
      direction                  = "Inbound"
      access                     = "Allow"
      protocol                   = "Tcp"
      source_port_range          = "*"
      destination_port_range     = security_rule.value.port
      source_address_prefix      = security_rule.value.source
      destination_address_prefix = "*"
    }
  }

  security_rule {
    name                       = "deny-all-inbound"
    priority                   = 4096
    direction                  = "Inbound"
    access                     = "Deny"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

resource "azurerm_subnet_network_security_group_association" "host" {
  subnet_id                 = azurerm_subnet.host.id
  network_security_group_id = azurerm_network_security_group.host.id
}

# A static address for the DNS name public_hostname, which the operator points at it.
resource "azurerm_public_ip" "host" {
  name                = local.name
  location            = var.location
  resource_group_name = local.resource_group_name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = local.tags
}

resource "azurerm_network_interface" "host" {
  name                = local.name
  location            = var.location
  resource_group_name = local.resource_group_name
  tags                = local.tags

  ip_configuration {
    name                          = "primary"
    subnet_id                     = azurerm_subnet.host.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.host.id
  }
}

resource "azurerm_network_interface_security_group_association" "host" {
  network_interface_id      = azurerm_network_interface.host.id
  network_security_group_id = azurerm_network_security_group.host.id
}

# ---- Host ------------------------------------------------------------------------------------

resource "azurerm_linux_virtual_machine" "host" {
  name                            = local.name
  location                        = var.location
  resource_group_name             = local.resource_group_name
  size                            = var.vm_size
  network_interface_ids           = [azurerm_network_interface.host.id]
  custom_data                     = base64encode(module.host.user_data)
  disable_password_authentication = true
  tags                            = local.tags

  # The maintenance user of generic-vm; cloud-init gives it every key of admin_ssh_public_keys,
  # Azure needs one of them for the administrator.
  admin_username = "bothy"
  admin_ssh_key {
    username   = "bothy"
    public_key = var.admin_ssh_public_keys[0]
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "StandardSSD_LRS"
    disk_size_gb         = 64
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = var.image_sku
    version   = "latest"
  }

  lifecycle {
    # The setup runs once at the first boot; new custom data must not replace the VM with its
    # data. A new setup goes over SSH (generic-vm provision_over_ssh).
    ignore_changes = [custom_data]
  }
}
