# idcd Ansible — Node Deployment

Automated provisioning and lifecycle management for idcd Agent nodes across
Hetzner, Vultr, RackNerd, DMIT, and BWG VPS providers.

## Prerequisites

| Requirement | Minimum version |
|---|---|
| Python | 3.10+ |
| Ansible | 2.15+ (`ansible-core`) |
| Target OS | Ubuntu 22.04 LTS (Jammy) |
| SSH access | Key-based auth to every node |

## Install Ansible dependencies

```bash
pip install "ansible-core>=2.15"
ansible-galaxy collection install community.general
```

## Directory layout

```
infra/ansible/
├── inventory/
│   ├── hosts.yml.example   # Template — copy and fill in real IPs
│   └── hosts.yml           # Your real inventory (gitignored)
├── group_vars/
│   └── all.yml             # Global variables
├── host_vars/              # Per-host overrides (optional)
├── templates/
│   ├── agent-config.yaml.j2
│   └── idcd-agent.service.j2
├── tasks/
│   └── update-agent.yml    # Shared update task list
├── site.yml                # System initialisation
├── agent.yml               # Agent deploy / re-deploy
└── agent-update.yml        # OTA canary update (L1 → L2 → L3)
```

## Quick-start

### 1. Create your inventory

```bash
cp infra/ansible/inventory/hosts.yml.example infra/ansible/inventory/hosts.yml
# Edit hosts.yml: add real IPs, ansible_user, node_id for each host
```

### 2. Verify connectivity

```bash
ansible -i infra/ansible/inventory/hosts.yml all -m ping
```

### 3. Initialise a fresh node (SSH hardening + UFW + fail2ban)

```bash
ansible-playbook -i infra/ansible/inventory/hosts.yml infra/ansible/site.yml
```

> **Note**: If you change `ssh_port` from 22, re-run subsequent commands with
> `-e "ansible_port=<new_port>"` or set it in `host_vars/`.

### 4. Deploy the agent

```bash
# Deploy current version from group_vars/all.yml
ansible-playbook -i infra/ansible/inventory/hosts.yml infra/ansible/agent.yml

# Deploy a specific version
ansible-playbook -i infra/ansible/inventory/hosts.yml infra/ansible/agent.yml \
  -e agent_version=0.2.0

# Deploy only tier-1 CN nodes
ansible-playbook -i infra/ansible/inventory/hosts.yml infra/ansible/agent.yml \
  --limit tier1_cn
```

## Common commands

### Run specific tags only

```bash
# Only apply SSH hardening
ansible-playbook -i inventory/hosts.yml site.yml --tags ssh

# Only write/reload config (no binary swap)
ansible-playbook -i inventory/hosts.yml agent.yml --tags config

# Only run health checks
ansible-playbook -i inventory/hosts.yml agent.yml --tags verify
```

### OTA canary update

```bash
# Canary update to 0.2.0 (interactive pauses between L1/L2/L3)
ansible-playbook -i inventory/hosts.yml agent-update.yml -e agent_version=0.2.0

# Non-interactive (CI / staging — skip pause prompts)
ansible-playbook -i inventory/hosts.yml agent-update.yml \
  -e agent_version=0.2.0 -e skip_pause=true

# Dry-run first
ansible-playbook -i inventory/hosts.yml agent-update.yml \
  --check -e agent_version=0.2.0
```

### Ad-hoc operations

```bash
# Check agent status on all nodes
ansible -i inventory/hosts.yml all -m command -a "systemctl status idcd-agent" --become

# Tail agent logs
ansible -i inventory/hosts.yml all -m command \
  -a "journalctl -u idcd-agent -n 50 --no-pager" --become

# Restart agent on a single host
ansible -i inventory/hosts.yml cn-beijing-01 \
  -m systemd -a "name=idcd-agent state=restarted" --become

# Check agent health endpoint
ansible -i inventory/hosts.yml all \
  -m uri -a "url=http://localhost:8080/health" --become
```

## Variable overrides

All variables in `group_vars/all.yml` can be overridden at multiple levels:

| Scope | File |
|---|---|
| Global default | `group_vars/all.yml` |
| Per tier | `group_vars/tier1_cn.yml` etc. |
| Per host | `host_vars/<hostname>.yml` |
| CLI one-off | `-e variable=value` |

Example `host_vars/cn-beijing-01.yml`:

```yaml
ssh_port: 2222
node_id: "nod_abc123xyz789"
node_region: "CN-Beijing-BGP"
node_isp: "DMIT-CN2-GIA"
```

## Target: 30-minute 100-node deployment

With a tuned control machine (8 cores) and `ansible.cfg`:

```ini
[defaults]
forks = 50
pipelining = true
```

`ansible-playbook agent.yml` runs 50 hosts in parallel, completing 100 nodes
in approximately 2 parallel batches (~15 min download + ~5 min verify/start).
