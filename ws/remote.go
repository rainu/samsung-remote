package ws

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"sync"
	"time"
)

type samsungRemote struct {
	applicationUrl string
	channelUrl     string
	b64Name        string
	wsDialer       *websocket.Dialer

	conlock           sync.RWMutex
	channelConnection *websocket.Conn
	conContext        context.Context
	conContextCancel  context.CancelFunc
}

type SamsungRemoteConfig struct {
	//The base url to be used for the websocket connection. Typically this is one of these:
	//
	// ws://tv_ip:8001
	//
	// wss://tv_ip:8002
	BaseUrl string

	//The name of the remote. This name will displayed at the TV.
	Name string

	//The token for the api. If no token is specified, the TV may always ask for permission.
	//The token is returning by the first connection.
	Token string

	//Optional. The websocket dialer to use.
	WebsocketDialer *websocket.Dialer
}

type SamsungApplication struct {
	Id   string `json:"appId"`
	Type int    `json:"app_type"`
	Name string `json:"name"`
}

type ApplicationStatus struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	IsRunning bool   `json:"running"`
	Version   string `json:"version"`
	IsVisible bool   `json:"visible"`
}

type ConnectionLostHandler func(error)

type SamsungRemote interface {
	//Establish a new connection to samsung tv.
	//It will return the token which should be stored and reused for future usages.
	//The given conLost-Handler will call when connection was lost.
	Connect(conLost ConnectionLostHandler) (string, error)

	//Establish a new connection to samsung tv.
	//It will return the token which should be stored and reused for future usages.
	//The given conLost-Handler will call when connection was lost.
	//Cancels the action if the passed context is closed.
	ConnectWithContext(conLost ConnectionLostHandler, ctx context.Context) (string, error)

	//Send a remote key.
	SendKey(key string) error
	//Send text.
	SendText(text []byte) error
	//Move the cursor to the given position.
	Move(x int, y int) error
	//Perform a left click.
	LeftClick() error
	//Perform a right click.
	RightClick() error
	//Open the built-in browser with the given url.
	OpenBrowser(url string) error

	//Get a list of all installed apps on the connected samsung tv.
	GetInstalledApps() ([]SamsungApplication, error)
	//Get a list of all installed apps on the connected samsung tv.
	//Cancels the action if the passed context is closed.
	GetInstalledAppsWithContext(ctx context.Context) ([]SamsungApplication, error)

	//Starts a app with the given app-id.
	StartApp(appId string) error
	//Starts a app with the given app-id.
	//Cancels the action if the passed context is closed.
	StartAppWithContext(appId string, ctx context.Context) error

	//Get the status by the given app id.
	GetAppStatus(appId string) (ApplicationStatus, error)
	//Get the status by the given app id.
	//Cancels the action if the passed context is closed.
	GetAppStatusWithContext(appId string, ctx context.Context) (ApplicationStatus, error)

	//Close the app by the given id.
	CloseApp(appId string) error
	//Close the app by the given id.
	//Cancels the action if the passed context is closed.
	CloseAppWithContext(appId string, ctx context.Context) error

	//Close the established connection an free the resources.
	Close() error
}

type wsMessage struct {
	Event string `json:"event"`
	Data  struct {
		Clients []struct {
			Attributes struct {
				Name  string `json:"name"`
				Token string `json:"token"`
			} `json:"attributes"`
		} `json:"clients"`
		Token string `json:"token"`
	} `json:"data"`
}

//Create a new instance of websocket-based SamsungRemote.
func NewRemote(conf SamsungRemoteConfig) SamsungRemote {
	if conf.Name == "" {
		conf.Name = "rainu-samsung-remote"
	}
	if conf.WebsocketDialer == nil {
		conf.WebsocketDialer = &websocket.Dialer{
			Proxy:            http.ProxyFromEnvironment,
			HandshakeTimeout: 10 * time.Second,
			TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		}
	}
	b64Name := base64.StdEncoding.EncodeToString([]byte(conf.Name))
	channelUrl := fmt.Sprintf("%s/api/v2/channels/samsung.remote.control?name=%s", conf.BaseUrl, b64Name)
	if conf.Token != "" {
		channelUrl = fmt.Sprintf("%s&token=%s", channelUrl, conf.Token)
	}

	return &samsungRemote{
		applicationUrl: fmt.Sprintf("%s/api/v2/", conf.BaseUrl),
		channelUrl:     channelUrl,
		b64Name:        b64Name,
		wsDialer:       conf.WebsocketDialer,
	}
}

func (s *samsungRemote) Connect(conLost ConnectionLostHandler) (string, error) {
	return s.ConnectWithContext(conLost, context.Background())
}

func (s *samsungRemote) ConnectWithContext(conLost ConnectionLostHandler, ctx context.Context) (string, error) {
	connection, _, err := s.wsDialer.DialContext(ctx, s.channelUrl, nil)
	if err != nil {
		return "", fmt.Errorf("error while connecting: %w", err)
	}
	s.channelConnection = connection

	rawMessage, err := s.readChannelMessage(ctx)
	if err != nil {
		return "", fmt.Errorf("error while read message: %w", err)
	}
	parsedMessage := wsMessage{}
	if err := json.Unmarshal(rawMessage, &parsedMessage); err != nil {
		return "", fmt.Errorf("invalid message received: %w", err)
	}

	if parsedMessage.Event != "ms.channel.connect" {
		return "", fmt.Errorf("unexpected message received: %s", rawMessage)
	}

	receivedToken := ""
	if parsedMessage.Data.Token != "" {
		receivedToken = parsedMessage.Data.Token
	} else {
		for _, client := range parsedMessage.Data.Clients {
			if client.Attributes.Name == s.b64Name {
				receivedToken = client.Attributes.Token
				break
			}
		}
	}
	if receivedToken != "" {
		//ensure the token is applied in internal channel-url
		if !strings.Contains(s.channelUrl, receivedToken) {
			//lazy but it works: append the right token (it does not matter if a token is alre//The given conLost-Handler will call when connection was lost.ady present in url)
			s.channelUrl += fmt.Sprintf(`&token=%s`, receivedToken)
		}
	}

	s.startConnectionWatcher(conLost)
	return receivedToken, nil
}

func (s *samsungRemote) startConnectionWatcher(conLost ConnectionLostHandler) {
	s.conContext, s.conContextCancel = context.WithCancel(context.Background())

	sendPing := func() error {
		s.conlock.Lock()
		defer s.conlock.Unlock()

		return s.channelConnection.WriteMessage(websocket.PingMessage, []byte(`ping`))
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				if err := sendPing(); err != nil {
					if conLost != nil {
						conLost(err) //call the connection lost handler
					}
					return
				}
			case <-s.conContext.Done():
				return
			}
		}
	}()
}

func (s *samsungRemote) SendKey(key string) error {
	message := fmt.Sprintf(`{"method":"ms.remote.control","params":{"Cmd":"Click","DataOfCmd":"%s","Option":"false","TypeOfRemote":"SendRemoteKey"}}`, key)
	return s.sendChannelMessage(message)
}

func (s *samsungRemote) Move(x, y int) error {
	message := fmt.Sprintf(`{"method":"ms.remote.control","params":{"Cmd":"Move","Position":{"x":%d,"y":%d,"Time":"%d"},"TypeOfRemote":"ProcessMouseDevice"}}`, x, y, time.Now().UnixNano()/1000/1000)
	return s.sendChannelMessage(message)
}

func (s *samsungRemote) LeftClick() error {
	message := `{"method":"ms.remote.control","params":{"Cmd":"LeftClick","TypeOfRemote":"ProcessMouseDevice"}}`
	return s.sendChannelMessage(message)
}

func (s *samsungRemote) RightClick() error {
	message := `{"method":"ms.remote.control","params":{"Cmd":"RightClick","TypeOfRemote":"ProcessMouseDevice"}}`
	return s.sendChannelMessage(message)
}

func (s *samsungRemote) SendText(text []byte) error {
	message := fmt.Sprintf(`{"method":"ms.remote.control","params":{"Cmd":"%s","TypeOfRemote":"SendInputString","DataOfCmd":"base64"}}`, base64.StdEncoding.EncodeToString(text))
	return s.sendChannelMessage(message)
}

func (s *samsungRemote) OpenBrowser(url string) error {
	message := fmt.Sprintf(`{"method":"ms.channel.emit","params":{"event":"ed.apps.launch","to":"host","data":{"appId":"org.tizen.browser","action_type":"NATIVE_LAUNCH","metaTag":"%s"}}}`, url)
	return s.sendChannelMessage(message)
}

func (s *samsungRemote) GetInstalledApps() ([]SamsungApplication, error) {
	return s.GetInstalledAppsWithContext(context.Background())
}

func (s *samsungRemote) GetInstalledAppsWithContext(ctx context.Context) ([]SamsungApplication, error) {
	message := `{"method":"ms.channel.emit","params":{"event":"ed.installedApp.get","to":"host"}}`
	readMessage, err := s.executeSingleCommand(s.channelUrl, message, ctx)
	if err != nil {
		return nil, err
	}

	result := struct {
		Data struct {
			Applications []SamsungApplication `json:"data"`
		} `json:"data"`
	}{}

	if err := json.Unmarshal(readMessage, &result); err != nil {
		return nil, fmt.Errorf("could not read message: %w", err)
	}
	return result.Data.Applications, nil
}

func (s *samsungRemote) StartApp(appId string) error {
	return s.StartAppWithContext(appId, context.Background())
}

func (s *samsungRemote) StartAppWithContext(appId string, ctx context.Context) error {
	message := fmt.Sprintf(`{"method":"ms.channel.emit","params":{"event":"ed.apps.launch","to":"host","data":{"appId":"%s","action_type":"DEEP_LINK"}}}`, appId)
	readMessage, err := s.executeSingleCommand(s.channelUrl, message, ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(string(readMessage), `"event":"ed.apps.launch"`) {
		return errors.New("app may not have been started")
	}
	return nil
}

func (s *samsungRemote) GetAppStatus(appId string) (ApplicationStatus, error) {
	return s.GetAppStatusWithContext(appId, context.Background())
}

func (s *samsungRemote) GetAppStatusWithContext(appId string, ctx context.Context) (ApplicationStatus, error) {
	msgId := fmt.Sprintf("%d", time.Now().Unix())
	message := fmt.Sprintf(`{"method":"ms.application.get","id":"%s","params":{"id":"%s"}}`, msgId, appId)
	readMessage, err := s.executeSingleCommand(s.applicationUrl, message, ctx)
	if err != nil {
		return ApplicationStatus{}, err
	}
	if !strings.Contains(string(readMessage), msgId) {
		return ApplicationStatus{}, errors.New("no valid status from server")
	}
	result := struct {
		Status ApplicationStatus `json:"result"`
	}{}
	if err := json.Unmarshal(readMessage, &result); err != nil {
		return ApplicationStatus{}, fmt.Errorf("could not decode response: %w", err)
	}
	return result.Status, nil
}

func (s *samsungRemote) CloseApp(appId string) error {
	return s.CloseAppWithContext(appId, context.Background())
}

func (s *samsungRemote) CloseAppWithContext(appId string, ctx context.Context) error {
	msgId := fmt.Sprintf("%d", time.Now().Unix())
	message := fmt.Sprintf(`{"method":"ms.application.stop","id":"%s","params":{"id":"%s"}}`, msgId, appId)
	readMessage, err := s.executeSingleCommand(s.applicationUrl, message, ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(string(readMessage), msgId) {
		return errors.New("no valid status from server")
	}
	return nil
}

func (s *samsungRemote) Close() error {
	s.conContextCancel() //cancel the connection watcher

	return s.channelConnection.Close()
}

// executeSingleCommand establish a fresh connection,
// sends the given message,
// returns the first read message,
// close the connection.
//
// This can be necessary because the "main-connection" will be polluted
// by some other event messages (such like update events).
func (s *samsungRemote) executeSingleCommand(url string, message string, ctx context.Context) ([]byte, error) {
	connection, _, err := s.wsDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error while connecting: %w", err)
	}
	defer connection.Close()

	rawMessage, err := readMessage(connection, ctx)
	if err != nil {
		return nil, fmt.Errorf("error while read message: %w", err)
	}
	parsedMessage := wsMessage{}
	if err := json.Unmarshal(rawMessage, &parsedMessage); err != nil {
		return nil, fmt.Errorf("invalid message received: %w", err)
	}
	if parsedMessage.Event != "ms.channel.connect" {
		return nil, fmt.Errorf("unexpected message received: %s", rawMessage)
	}

	//connections established
	if err := sendMessage(connection, message); err != nil {
		return nil, err
	}

	return readMessage(connection, ctx)
}

func (s *samsungRemote) sendChannelMessage(message string) error {
	s.conlock.Lock()
	defer s.conlock.Unlock()

	return sendMessage(s.channelConnection, message)
}

func sendMessage(con *websocket.Conn, message string) error {
	if con == nil {
		return errors.New("connection is not established")
	}

	err := con.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		return fmt.Errorf("error while writing message: %w", err)
	}

	return nil
}

type readMessageResult struct {
	messageType int
	message     []byte
	err         error
}

func (s *samsungRemote) readChannelMessage(ctx context.Context) ([]byte, error) {
	return readMessage(s.channelConnection, ctx)
}

func readMessage(con *websocket.Conn, ctx context.Context) ([]byte, error) {
	messageChan := make(chan readMessageResult, 1)
	go func() {
		defer close(messageChan)

		result := readMessageResult{}

		result.messageType, result.message, result.err = con.ReadMessage()
		messageChan <- result
	}()

	select {
	case result := <-messageChan:
		return result.message, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("context was closed: %w", ctx.Err())
	}
}
