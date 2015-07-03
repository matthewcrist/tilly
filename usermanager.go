package main

import (
	"github.com/nlopes/slack"
	"log"
)

type UserManager struct {
	client             *AuthedSlack
	messageReplies     chan slack.MessageEvent
	newStandups        chan newStandupForUser
	usersByUserId      map[string]*User
	usersByIMChannelId map[string]*User
}

type newStandupForUser struct {
	standup *Standup
	userId  string
}

func NewUserManager(client *AuthedSlack) (um *UserManager) {
	um = &UserManager{
		client:             client,
		messageReplies:     make(chan slack.MessageEvent),
		newStandups:        make(chan newStandupForUser),
		usersByUserId:      make(map[string]*User),
		usersByIMChannelId: make(map[string]*User),
	}
	go um.start()
	return
}

func (self *UserManager) StartStandup(s *Standup, userId string) {
	self.newStandups <- newStandupForUser{standup: s, userId: userId}
}

func (self *UserManager) ReceiveMessageReply(m slack.MessageEvent) {
	self.messageReplies <- m
}

func (self *UserManager) start() {
	var user *User
	var err error
	var ok bool

	for {
		user = nil

		select {
		case m := <-self.messageReplies:
			if user, ok = self.usersByIMChannelId[m.ChannelId]; !ok {
				user, err = self.lookupUserByIMChannelId(m.ChannelId)
				if err != nil {
					log.Printf(
						"error getting channel info; message dropped: %s", err)
					continue
				}
				if user == nil {
					continue
				} else {
					self.usersByUserId[user.Info.Id] = user
					self.usersByIMChannelId[m.ChannelId] = user
				}
			}
			user.ReceiveMessageReply(m)

		case ns := <-self.newStandups:
			if user, ok = self.usersByUserId[ns.userId]; !ok {
				user, err = self.lookupUserById(ns.userId)
				if err != nil {
					log.Printf("error getting user info; new standup dropped: %s", err)
					continue
				}
				if user == nil {
					continue
				} else {
					self.usersByUserId[ns.userId] = user
					self.usersByIMChannelId[user.imChannelId] = user
				}
			}
			user.StartStandup(ns.standup)
		}
	}
}

func (self *UserManager) lookupUserByIMChannelId(channelId string) (user *User, err error) {
	ims, err := self.client.GetIMChannels()
	if err != nil {
		return
	}
	for _, im := range ims {
		if channelId != "" && im.Id == channelId {
			return self.newUser(im.UserId, im.Id)
		}
	}
	return
}

func (self *UserManager) lookupUserById(userId string) (user *User, err error) {
	_, _, channelId, err := self.client.OpenIMChannel(userId)
	if err != nil {
		return nil, err
	}
	return self.newUser(userId, channelId)
}

func (self *UserManager) newUser(userId string, imChannelId string) (user *User, err error) {
	userInfo, err := self.client.GetUserInfo(userId)
	if err != nil {
		return nil, err
	}
	return NewUser(self.client, *userInfo, imChannelId), nil
}