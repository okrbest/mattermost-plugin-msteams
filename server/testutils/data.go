package testutils

import (
	"net/http"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-server/v6/model"
)

func GetTeamID() string {
	return "pqoeurndhroajdemq4nfmw"
}

func GetChannelID() string {
	return "bnqnzipmnir4zkkj95ggba5pde"
}

func GetSenderID() string {
	return "pqoejrn65psweomewmosaqr"
}

func GetUserID() string {
	return "sfmq19kpztg5iy47ebe51hb31w"
}

func GetTeamsUserID() string {
	return "rioegneonqimomsoqwiew3qeo"
}

func GetMessageID() string {
	return "dsdfonreoapwer4onebfdr"
}

func GetReplyID() string {
	return "pqoeurndhroajdemq4nfmw"
}

func GetChatID() string {
	return "qplsnwere9nurernidteoqw"
}

func GetMattermostID() string {
	return "qwifdnaootmgkerodfdmwo"
}

func GetPostID() string {
	return "qwifdnaootmgkerodfdmwo"
}

func GetInternalServerAppError(errorMsg string) *model.AppError {
	return &model.AppError{
		StatusCode:    http.StatusInternalServerError,
		DetailedError: errorMsg,
		Id:            GetID(),
	}
}

func GetID() string {
	return "sfmq19kpztg5iy47ebe51hb31w"
}

func GetMSTeamsChannelID() string {
	return "qplsnwere9nurernidteoqw"
}

func GetPost(channelID, userID string, createAt int64) *model.Post {
	return &model.Post{
		Id:        GetID(),
		FileIds:   model.StringArray{GetID()},
		ChannelId: channelID,
		UserId:    userID,
		CreateAt:  createAt,
	}
}

func GetChannel(channelType model.ChannelType) *model.Channel {
	return &model.Channel{
		Id:   GetChannelID(),
		Type: channelType,
	}
}

func GetChannelMembers(count int) model.ChannelMembers {
	channelMembers := model.ChannelMembers{}
	for i := 0; i < count; i++ {
		channelMembers = append(channelMembers, model.ChannelMember{
			UserId:    GetID(),
			ChannelId: GetChannelID(),
		})
	}

	return channelMembers
}

func GetUser(role, email string) *model.User {
	return &model.User{
		Id:       GetID(),
		Username: "test-user",
		Roles:    role,
		Email:    email,
	}
}

func GetReaction() *model.Reaction {
	return &model.Reaction{
		EmojiName: "+1",
		UserId:    GetID(),
		PostId:    GetID(),
		ChannelId: GetChannelID(),
	}
}

func GetTeamsTeamID() string {
	return "test-teams-team"
}

func GetTeamsChannelID() string {
	return "test-teams-channel"
}

func GetChannelLinks(count int) []*storemodels.ChannelLink {
	var links []*storemodels.ChannelLink
	for i := 0; i < count; i++ {
		links = append(links, &storemodels.ChannelLink{
			MattermostTeamID:      GetTeamID(),
			MattermostChannelID:   GetChannelID(),
			MattermostTeamName:    "Test MM team",
			MattermostChannelName: "Test MM channel",
			MSTeamsTeam:           GetTeamsTeamID(),
			MSTeamsChannel:        GetTeamsChannelID(),
		})
	}

	return links
}

func GetConnectedUsers(count int) []*storemodels.ConnectedUser {
	var connectedUsers []*storemodels.ConnectedUser
	for i := 0; i < count; i++ {
		connectedUsers = append(connectedUsers, &storemodels.ConnectedUser{
			MattermostUserID: GetUserID(),
			TeamsUserID:      GetTeamsUserID(),
		})
	}

	return connectedUsers
}

func GetFileInfo() *model.FileInfo {
	return &model.FileInfo{
		Id:       GetID(),
		Name:     "mockFile.Name.txt",
		Size:     1,
		MimeType: "mockMimeType",
	}
}

func GetPostFromTeamsMessage(createAt int64) *model.Post {
	return &model.Post{
		UserId:    GetUserID(),
		ChannelId: GetChannelID(),
		Message:   "mockText",
		CreateAt:  createAt,
		Props: model.StringInterface{
			"msteams_sync_mock-BotUserID": true,
		},
		FileIds: model.StringArray{},
	}
}

func GetTestEmail() string {
	return "unknown-user@msteamssync"
}
