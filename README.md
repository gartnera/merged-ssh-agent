# merged-ssh-agent

This tool merged multiple ssh agents. Example:

```
export SSH_AUTH_SOCKS=$HOME/.ssh/agent.sock,$HOME/.sekey/ssh-agent.ssh
export SSH_AGENT_MERGED_SOCK=$HOME/.ssh/merged-agent.sock

merged-ssh-agent &

SSH_AUTH_SOCK=$SSH_AGENT_MERGED_SOCK ssh test@example.com
```

Binaries for amd64/{darwin/linux} are avaliable via [gitlab CI](/pipelines).

Install via `go`:

```
go get -u gitlab.com/gartnera/merged-ssh-agent/cmd/merged-ssh-agent
```