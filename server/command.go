package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
)

const msteamsCommand = "msteamssync"
const msteamsLogoURL = "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg"

func createMsteamsSyncCommand() *model.Command {
	return &model.Command{
		Trigger:          msteamsCommand,
		AutoComplete:     true,
		AutoCompleteDesc: "Manage synced channels between MS Teams and Mattermost",
		AutoCompleteHint: "[command]",
		Username:         botUsername,
		IconURL:          msteamsLogoURL,
		DisplayName:      botDisplayName,
		AutocompleteData: getAutocompleteData(),
	}
}

func cmdError(channelID string, detailedError string) (*model.CommandResponse, *model.AppError) {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		ChannelId:    channelID,
		Text:         detailedError,
		Username:     botDisplayName,
		IconURL:      msteamsLogoURL,
	}, nil
}

func (p *Plugin) sendBotEphemeralPost(userID, channelID, message string) {
	p.API.SendEphemeralPost(userID, &model.Post{
		Message:   message,
		UserId:    p.botID,
		ChannelId: channelID,
	})
}

func getAutocompleteData() *model.AutocompleteData {
	cmd := model.NewAutocompleteData(msteamsCommand, "[command]", "Manage MS Teams linked channels")

	link := model.NewAutocompleteData("link", "[msteams-team-id] [msteams-channel-id]", "Link current channel to a MS Teams channel")
	link.AddTextArgument("MS Teams Team ID", "[msteams-team-id]", "")
	link.AddTextArgument("MS Teams Channel ID", "[msteams-channel-id]", "")
	cmd.AddCommand(link)

	unlink := model.NewAutocompleteData("unlink", "", "Unlink the current channel from the MS Teams channel")
	cmd.AddCommand(unlink)

	show := model.NewAutocompleteData("show", "", "Show MS Teams linked channel")
	cmd.AddCommand(show)

	return cmd
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	command := split[0]
	var parameters []string
	action := ""
	if len(split) > 1 {
		action = split[1]
	}
	if len(split) > 2 {
		parameters = split[2:]
	}

	if command != "/"+msteamsCommand {
		return &model.CommandResponse{}, nil
	}

	if action == "link" {
		return p.executeLinkCommand(c, args, parameters)
	}

	if action == "unlink" {
		return p.executeUnlinkCommand(c, args)
	}

	if action == "show" {
		return p.executeShowCommand(c, args)
	}

	return cmdError(args.ChannelId, "Unknown command. Valid options: link, unlink and show.")
}

func (p *Plugin) executeLinkCommand(c *plugin.Context, args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	if len(parameters) < 2 {
		return cmdError(args.ChannelId, "Invalid link command, please pass the MS Teams team id and channel id as parameters.")
	}

	if !p.checkEnabledTeamByTeamId(args.TeamId) {
		return cmdError(args.ChannelId, "This team is not enabled for MS Teams sync.")
	}

	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the current channel information.")
	}

	canLinkChannel := channel.Type == model.ChannelTypeOpen && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePublicChannelProperties)
	canLinkChannel = canLinkChannel || (channel.Type == model.ChannelTypePrivate && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePrivateChannelProperties))
	if !canLinkChannel {
		return cmdError(args.ChannelId, "Unable to link the channel. You have to be a channel admin to link it.")
	}

	_, ok := p.channelsLinked[channel.TeamId+":"+channel.Id]
	if ok {
		return cmdError(args.ChannelId, "A link for this channel already exists, please remove unlink the channel before you link a new one")
	}

	_, err := p.msteamsAppClient.GetChannel(parameters[0], parameters[1])
	if err != nil {
		return cmdError(args.ChannelId, "MS Teams channel not found.")
	}

	link := ChannelLink{
		MattermostTeam:    channel.TeamId,
		MattermostChannel: channel.Id,
		MSTeamsTeam:       parameters[0],
		MSTeamsChannel:    parameters[1],
	}
	p.channelsLinked[channel.TeamId+":"+channel.Id] = link

	subscriptionID, err := p.subscribeToChannel(link)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to subscribe to channel, probably it is already link to another channel.")
	}

	go p.refreshSubscriptionPeridically(p.stopContext, subscriptionID)

	err = p.saveChannelsLinked()
	if err != nil {
		return cmdError(args.ChannelId, "Unable to store the new link, please try again.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is now linked to this Mattermost channel")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeUnlinkCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the current channel information.")
	}

	canLinkChannel := channel.Type == model.ChannelTypeOpen && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePublicChannelProperties)
	canLinkChannel = canLinkChannel || (channel.Type == model.ChannelTypePrivate && p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManagePrivateChannelProperties))
	if !canLinkChannel {
		return cmdError(args.ChannelId, "Unable to unlink the channel, you has to be a channel admin to unlink it.")
	}

	link, ok := p.channelsLinked[channel.TeamId+":"+channel.Id]
	if !ok {
		return cmdError(args.ChannelId, "Link doesn't exist.")
	}

	err := p.unsubscribeFromChannel(link)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to store unsubscribe from channel, please try again.")
	}

	delete(p.channelsLinked, channel.TeamId+":"+channel.Id)

	if err = p.saveChannelsLinked(); err != nil {
		return cmdError(args.ChannelId, "Unable to store the new link, please try again.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is no longer linked to this Mattermost channel")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeShowCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return cmdError(args.ChannelId, "Unable to get the current channel information.")
	}

	link, ok := p.channelsLinked[channel.TeamId+":"+channel.Id]
	if !ok {
		return cmdError(args.ChannelId, "Link doesn't exists.")
	}

	msteamsTeam, err := p.msteamsAppClient.GetTeam(link.MSTeamsTeam)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to get the MS Teams team information.")
	}

	msteamsChannel, err := p.msteamsAppClient.GetChannel(link.MSTeamsTeam, link.MSTeamsChannel)
	if err != nil {
		return cmdError(args.ChannelId, "Unable to get the MS Teams channel information.")
	}

	text := fmt.Sprintf(
		"This channel is linked to the MS Teams Channel \"%s\" (with id: %s) in the Team \"%s\" (with the id: %s).",
		msteamsChannel.DisplayName,
		msteamsChannel.ID,
		msteamsTeam.DisplayName,
		msteamsTeam.ID,
	)

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, text)
	return &model.CommandResponse{}, nil
}
