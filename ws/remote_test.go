package ws

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRemote(t *testing.T) {
	remote := NewRemote(SamsungRemoteConfig{
		BaseUrl: "wss://tv_ip:8002",
		Name:    "Rainu",
		Token:   "My_TOKEN",
	})

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	token, err := remote.ConnectWithContext(nil, ctx)
	assert.NoError(t, err)

	fmt.Printf("%s\n", token)
	assert.NotEmpty(t, token)

	assert.NoError(t, remote.SendKey("KEY_VOLDOWN"))

	assert.NoError(t, remote.OpenBrowser(""))
	assert.NoError(t, remote.Move(50, 10))
	assert.NoError(t, remote.SendText([]byte(`raysha.de`)))
	apps, err := remote.GetInstalledApps()
	assert.NoError(t, err)
	fmt.Printf("%#v\n", apps)

	for _, app := range apps {
		if app.Name == "YouTube" {
			assert.NoError(t, remote.StartApp(app.Id))
		}
	}

	status, err := remote.GetAppStatus("111299001912")
	assert.NoError(t, err)
	fmt.Printf("%#v", status)
}
