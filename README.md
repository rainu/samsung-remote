# samsung-remote
A golang library for remote controlling Samsung televisions via http(s) or websocket connection.

There are two ways to control the Samsung TV: via HTTP or via websockets. Each interface can do a little more or less 
than the other. The HTTP interface is synchronous. The WS interface is asynchronous. The HTTP interface can be used
without authorization. With the WS interface, permission may need to be granted in Samsung TV. Furthermore, I have 
observed with my TV that this only works with the WSS interface!

# usage
```go
package main

import (
	"fmt"
	samsungRemoteWS "github.com/rainu/samsung-remote/ws"
)

func main() {
	remote := samsungRemoteWS.NewRemote(samsungRemoteWS.SamsungRemoteConfig{
		BaseUrl: "wss://tv_ip:8002",
		Name:    "rainu-samsung-remote",
	})
	defer remote.Close()

	token, err := remote.Connect(nil)
	if err != nil {
		panic(err)
 	}

	//save the token anywhere and make sure this token will be set in future
	fmt.Printf("Token: %s", token)

	err = remote.SendKey("KEY_MENU")
	if err != nil {
		panic(err)
	}
}
```

# key codes

The list of accepted keys may vary depending on the TV model, but the following list has some common key codes and their descriptions.

|Key code| Description |
|------|----|
|KEY_POWEROFF       |Power off|
|KEY_UP             |Up|
|KEY_DOWN           |Down|
|KEY_LEFT           |Left|
|KEY_RIGHT          |Right|
|KEY_CHUP           |P Up|
|KEY_CHDOWN         |P Down|
|KEY_ENTER          |Enter|
|KEY_RETURN         |Return|
|KEY_CH_LIST        |Channel List|
|KEY_MENU           |Menu|
|KEY_SOURCE         |Source|
|KEY_GUIDE          |Guide|
|KEY_TOOLS          |Tools|
|KEY_INFO           |Info|
|KEY_RED            |A / Red|
|KEY_GREEN          |B / Green|
|KEY_YELLOW         |C / Yellow|
|KEY_BLUE           |D / Blue|
|KEY_PANNEL_CHDOWN  |3D|
|KEY_VOLUP          |Volume Up|
|KEY_VOLDOWN        |Volume Down|
|KEY_MUTE           |Mute|
|KEY_0              |0|
|KEY_1              |1|
|KEY_2              |2|
|KEY_3              |3|
|KEY_4              |4|
|KEY_5              |5|
|KEY_6              |6|
|KEY_7              |7|
|KEY_8              |8|
|KEY_9              |9|
|KEY_DTV            |TV Source|
|KEY_HDMI           |HDMI Source|
|KEY_CONTENTS       |SmartHub|

# Honor

Since this API does not seem to be documented anywhere, I had to look in different repositories :)

* https://github.com/Ape/samsungctl
* https://github.com/Ape/samsungctl/issues/75
* https://gist.github.com/freman/8d98742de09d476c4d3d9e5d55f9db63