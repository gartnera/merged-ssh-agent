package main

import (
	agent "gitlab.com/gartnera/merged-ssh-agent"
)

func main() {
	agent.ServeEnv()
}
