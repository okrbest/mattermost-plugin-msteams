package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-matterbridge/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const (
	botUsername    = "msteams"
	botDisplayName = "MS Teams"
	pluginID       = "com.mattermost.matterbridge-plugin"
)

type ChannelLink struct {
	MattermostTeam    string
	MattermostChannel string
	MSTeamsTeam       string
	MSTeamsChannel    string
}

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	msteamsAppClientMutex sync.Mutex
	msteamsAppClient      *msteams.Client
	msteamsBotClientMutex sync.Mutex
	msteamsBotClient      *msteams.Client

	botID  string
	userID string

	subscriptionsToLinksMutex sync.Mutex
	subscriptionsToLinks      map[string]ChannelLink
	channelsLinked            map[string]ChannelLink

	stopSubscriptions func()
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()
	router.HandleFunc("/avatar/{userId:.*}", p.getAvatar).Methods("GET")
	router.HandleFunc("/", p.processMessage).Methods("GET", "POST")
	router.ServeHTTP(w, r)
}

func (p *Plugin) getURL() string {
	config := p.API.GetConfig()
	if strings.HasSuffix(*config.ServiceSettings.SiteURL, "/") {
		return *config.ServiceSettings.SiteURL + "plugins/" + pluginID
	}
	return *config.ServiceSettings.SiteURL + "/plugins/" + pluginID
}

func (p *Plugin) connectTeamsAppClient() error {
	p.msteamsAppClientMutex.Lock()
	botClient, err := msteams.NewApp(
		p.configuration.TenantId,
		p.configuration.ClientId,
		p.configuration.ClientSecret,
	)
	if err != nil {
		return err
	}
	p.msteamsAppClient = botClient
	p.msteamsAppClientMutex.Unlock()
	return nil
}

func (p *Plugin) connectTeamsBotClient() error {
	p.msteamsBotClientMutex.Lock()
	botClient, err := msteams.NewBot(
		p.configuration.TenantId,
		p.configuration.ClientId,
		p.configuration.ClientSecret,
		p.configuration.BotUsername,
		p.configuration.BotPassword,
	)
	if err != nil {
		return err
	}
	p.msteamsBotClient = botClient
	p.msteamsBotClientMutex.Unlock()
	return nil
}

func (p *Plugin) start() {
	err := p.connectTeamsAppClient()
	if err != nil {
		p.API.LogError("Unable to connect to the msteams", "error", err)
		return
	}
	err = p.connectTeamsBotClient()
	if err != nil {
		p.API.LogError("Unable to connect to the msteams", "error", err)
		return
	}

	channelsLinkedData, appErr := p.API.KVGet("channelsLinked")
	if appErr != nil {
		p.API.LogError("Error getting the channels linked", "error", appErr)
		return
	}
	channelsLinked := map[string]ChannelLink{}
	err = json.Unmarshal(channelsLinkedData, &channelsLinked)
	if err != nil {
		p.API.LogError("Error getting the channels linked", "error", err)
		return
	}

	p.channelsLinked = channelsLinked
	p.subscriptionsToLinks = map[string]ChannelLink{}
	ctx, stop := context.WithCancel(context.Background())
	p.stopSubscriptions = stop
	err = p.clearSubscriptions()
	if err != nil {
		p.API.LogError("Unable to clear all subscriptions", "error", err)
	}
	for _, link := range channelsLinked {
		go p.subscribeToChannel(ctx, link)
	}
}

func (p *Plugin) stop() {
	if p.stopSubscriptions != nil {
		p.stopSubscriptions()
	}
}

func (p *Plugin) restart() {
	p.stop()
	p.start()
}

func (p *Plugin) OnActivate() error {
	p.stopSubscriptions = func() {}

	bot, appErr := p.API.CreateBot(&model.Bot{
		Username:    botUsername,
		DisplayName: botDisplayName,
		Description: "Created by the MS Teams Sync plugin.",
	})
	if appErr != nil {
		bot, err := p.API.GetUserByUsername(botUsername)
		if err != nil {
			return err
		}
		p.userID = bot.Id
	} else {
		p.userID = bot.UserId
	}

	if p.configuration.Config == "" {
		return nil
	}

	p.API.RegisterCommand(createMsteamsSyncCommand())

	go p.start()
	return nil
}

func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if post.Props != nil {
		if _, ok := post.Props["matterbridge_"+p.userID].(bool); ok {
			return
		}
	}

	channel, _ := p.API.GetChannel(post.ChannelId)

	link, ok := p.channelsLinked[channel.TeamId+":"+post.ChannelId]
	if !ok {
		return
	}

	user, _ := p.API.GetUser(post.UserId)

	go p.Send(link, user, post)
}

func (p *Plugin) OnDeactivate() error {
	p.stop()
	return nil
}

func (p *Plugin) Send(link ChannelLink, user *model.User, post *model.Post) (string, error) {
	p.API.LogDebug("\n\n\n=> Receiving message", "post", post)

	// TODO: Replace this with a template
	text := user.Username + "@mattermost: " + post.Message

	parentID := []byte{}
	if post.RootId != "" {
		parentID, _ = p.API.KVGet("mattermost_teams_" + post.RootId)
	}

	newMessageId, err := p.msteamsBotClient.SendMessage(link.MSTeamsTeam, link.MSTeamsChannel, string(parentID), text)
	if err != nil {
		p.API.LogError("Error creating post", "error", err)
		return "", err
	}

	if post.Id != "" && newMessageId != "" {
		p.API.KVSet("mattermost_teams_"+post.Id, []byte(newMessageId))
		p.API.KVSet("teams_mattermost_"+newMessageId, []byte(post.Id))
	}
	return newMessageId, nil
}

func (p *Plugin) clearSubscriptions() error {
	err := p.msteamsAppClient.ClearSubscriptions()
	if err != nil {
		p.API.LogError("subscription deletion failed", "error", err)
		return err
	}
	return nil
}

func (p *Plugin) subscribeToChannel(ctx context.Context, link ChannelLink) error {
	teamId := link.MSTeamsTeam
	channelId := link.MSTeamsChannel
	notificationURL := p.getURL() + "/"

	subscriptionID, err := p.msteamsAppClient.SubscribeToChannel(teamId, channelId, notificationURL)
	if err != nil {
		return err
	}
	p.subscriptionsToLinksMutex.Lock()
	p.subscriptionsToLinks[subscriptionID] = link
	p.subscriptionsToLinksMutex.Unlock()

	err = p.msteamsAppClient.RefreshSubscriptionPeriodically(ctx, subscriptionID)
	if err != nil {
		p.API.LogError("error updating subscription", "error", err)
		return err
	}

	return nil
}
