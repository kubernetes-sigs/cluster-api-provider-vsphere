package context

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/go-logr/logr"
)

type SSH struct {
	Logger logr.Logger
	Host   string
	User   string
	Pass   string
}

// Exec executes a command, forwarding the remote stderr/stdout
func (s *SSH) Exec(cmd string, args ...interface{}) error {
	cmd = fmt.Sprintf(cmd, args...)
	config := &ssh.ClientConfig{
		User: s.User,
		Auth: []ssh.AuthMethod{
			// ssh.Password needs to be explicitly allowed, and by default ESXi only allows public + keyboard interactive
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				// Just send the password back for all questions
				answers := make([]string, len(questions))
				for i, _ := range answers {
					answers[i] = s.Pass
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	s.Logger.V(2).Info(fmt.Sprintf("Connecting to %s@%s", s.User, s.Host))
	conn, err := ssh.Dial("tcp", s.Host, config)
	if err != nil {
		return err
	}
	defer conn.Close()

	var sess *ssh.Session
	var sessStdOut, sessStderr io.Reader
	if sess, err = conn.NewSession(); err != nil {
		return err
	}

	defer sess.Close()
	if sessStdOut, err = sess.StdoutPipe(); err != nil {
		return err
	}
	go io.Copy(os.Stdout, sessStdOut)
	if sessStderr, err = sess.StderrPipe(); err != nil {
		return err
	}
	go io.Copy(os.Stderr, sessStderr)
	return sess.Run(cmd)
}
