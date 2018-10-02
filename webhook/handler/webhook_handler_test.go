package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_get_radix_operator_repo_ssh_url_by_ping_url(t *testing.T) {
	pingURL := "https://api.github.com/repos/Statoil/radix-operator/hooks/50561858"
	sshURL := getSSHUrlFromPingURL(pingURL)

	assert.Equal(t, "git@github.com:Statoil/radix-operator.git", sshURL)
}

func Test_get_priv_repo_ssh_url_by_ping_url(t *testing.T) {
	pingURL := "https://api.github.com/repos/keaaa/go-roman/hooks/9917077"
	sshURL := getSSHUrlFromPingURL(pingURL)

	assert.Equal(t, "git@github.com:keaaa/go-roman.git", sshURL)
}
