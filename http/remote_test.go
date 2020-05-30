package http

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAppLauncher(t *testing.T) {
	conf := SamsungRemoteConfig{
		BaseUrl: "http://tv_ip:8001",
	}
	launcher := NewRemote(conf)

	information, err := launcher.GetInformation()
	assert.NoError(t, err)
	fmt.Printf("%s", information)

	assert.NoError(t, launcher.StartApp("Netflix", nil))
	assert.NoError(t, launcher.StartApp("YouTube", []byte(`v=OccUo_rqfhQ`)))

	status, err := launcher.GetAppStatus("111299001912")
	assert.NoError(t, err)
	fmt.Printf("%#v", status)

	assert.NoError(t, launcher.CloseApp("111299001912"))
}
