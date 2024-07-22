package agent

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"gitlab.com/gartnera/merged-ssh-agent/reconnect"
	"golang.org/x/crypto/ssh"
	sshAgent "golang.org/x/crypto/ssh/agent"
)

func WithBannedKeys(keys []string) func(*MergedAgent) {
	return func(m *MergedAgent) {
		m.bannedKeys = keys
	}
}

func agentKeyToStr(key *sshAgent.Key) string {
	return strings.ReplaceAll(string(ssh.MarshalAuthorizedKey(key)), "\n", " ") + key.Comment
}

func agentListKeysBanned(agent sshAgent.Agent, bannedKeys []string) ([]*sshAgent.Key, error) {
	var keys []*sshAgent.Key
	agentKeys, err := agent.List()
	if err != nil {
		return keys, err
	}
	for _, key := range agentKeys {
		keyStr := agentKeyToStr(key)
		ok := true
		for _, bannedKey := range bannedKeys {
			if bannedKey == keyStr {
				ok = false
			}
		}
		if !ok {
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

type MergedAgent struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         *sync.WaitGroup
	agents     []sshAgent.Agent
	bannedKeys []string
}

func (a MergedAgent) List() ([]*sshAgent.Key, error) {
	var keys []*sshAgent.Key
	for _, agent := range a.agents {
		agentKeys, err := agentListKeysBanned(agent, a.bannedKeys)
		if err != nil {
			return keys, err
		}
		keys = append(keys, agentKeys...)
	}
	return keys, nil
}

func (a MergedAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	// find the correct agent to request signing from
	for _, agent := range a.agents {
		agentKeys, err := agentListKeysBanned(agent, a.bannedKeys)
		if err != nil {
			return nil, err
		}
		for _, aKey := range agentKeys {
			if bytes.Equal(key.Marshal(), aKey.Marshal()) {
				return agent.Sign(key, data)
			}
		}
	}
	return nil, fmt.Errorf("no agent found for key: %s", ssh.MarshalAuthorizedKey(key))
}

func (a MergedAgent) Add(key sshAgent.AddedKey) error {
	fmt.Println("Error: Add() not implemented")
	return nil
}

func (a MergedAgent) Remove(key ssh.PublicKey) error {
	fmt.Println("Error: Remove() not implemented")
	return nil
}

func (a MergedAgent) RemoveAll() error {
	fmt.Println("Error: RemoveAll() not implemented")
	return nil
}

func (a MergedAgent) Lock(passphrase []byte) error {
	fmt.Println("Error: Lock() not implemented")
	return nil
}

func (a MergedAgent) Unlock(passphrase []byte) error {
	fmt.Println("Error: Unlock() not implemented")
	return nil
}

func (a MergedAgent) Signers() ([]ssh.Signer, error) {
	var signers []ssh.Signer
	for _, agent := range a.agents {
		s, err := agent.Signers()
		if err != nil {
			return signers, err
		}
		signers = append(signers, s...)
	}
	return signers, nil
}

func (a MergedAgent) Serve(conn net.Conn) error {
	a.wg.Add(1)
	defer a.wg.Done()
	go func() {
		a.wg.Add(1)
		defer a.wg.Done()
		<-a.ctx.Done()
		conn.Close()
	}()

	return sshAgent.ServeAgent(a, conn)
}

func (a MergedAgent) ListenAndServe(path string) error {
	l, err := net.Listen("unix", path)
	if err != nil {
		return err
	}

	go func() {
		a.wg.Add(1)
		defer a.wg.Done()
		<-a.ctx.Done()
		l.Close()
	}()

	go func() {
		a.wg.Add(1)
		defer a.wg.Done()
		for {
			conn, err := l.Accept()
			if err != nil {
				l.Close()
				a.cancel()
				return
			}

			go a.Serve(conn)
		}
	}()
	return nil
}

func (a MergedAgent) Close() {
	a.cancel()
	a.wg.Wait()
}

func NewMergedAgent(agents []sshAgent.Agent, options ...func(*MergedAgent)) MergedAgent {
	ctx, cancel := context.WithCancel(context.Background())
	a := MergedAgent{
		ctx:    ctx,
		cancel: cancel,
		wg:     &sync.WaitGroup{},
	}
	for _, agent := range agents {
		a.agents = append(a.agents, agent)
	}
	for _, opt := range options {
		opt(&a)
	}
	return a
}

func NewMergedAgentFromPaths(paths []string, options ...func(*MergedAgent)) (MergedAgent, error) {
	var agents []sshAgent.Agent
	for _, path := range paths {
		conn, err := reconnect.Dial("unix", path)
		if err != nil {
			return MergedAgent{}, err
		}
		agent := sshAgent.NewClient(conn)
		agents = append(agents, agent)
	}
	return NewMergedAgent(agents, options...), nil
}

func ServeEnv() {
	socketsVar := os.Getenv("SSH_AUTH_SOCKS")
	listenPath := os.Getenv("SSH_AUTH_SOCK_MERGED")
	if socketsVar == "" {
		log.Fatalln("Error: $SSH_AUTH_SOCKS required")
	}
	if listenPath == "" {
		log.Fatalln("Error: $SSH_AUTH_SOCK_MERGED required")
	}
	bannedVar := os.Getenv("BANNED_KEYS")
	bannedKeys := strings.Split(bannedVar, ",")
	sockets := strings.Split(socketsVar, ",")
	agent, err := NewMergedAgentFromPaths(sockets, WithBannedKeys(bannedKeys))
	if err != nil {
		panic(err)
	}
	err = agent.ListenAndServe(listenPath)
	if err != nil {
		panic(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigs:
		agent.Close()
	case <-agent.ctx.Done():
	}
}
