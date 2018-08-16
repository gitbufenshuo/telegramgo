package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gitbufenshuo/mtproto"
)

const updatePeriod = time.Second * 2

type Command struct {
	Name      string
	Arguments string
}

// Returns user nickname in two formats:
// <id> <First name> @<Username> <Last name> if user has username
// <id> <First name> <Last name> otherwise
func nickname(user mtproto.TL_user) string {
	if user.Username == "" {
		return fmt.Sprintf("%d %s %s", user.Id, user.First_name, user.Last_name)
	}

	return fmt.Sprintf("%d %s @%s %s", user.Id, user.First_name, user.Username, user.Last_name)
}

// Returns date in RFC822 format
func formatDate(date int32) string {
	unixTime := time.Unix((int64)(date), 0)
	return unixTime.Format(time.RFC822)
}

// Show help
func help() {
	fmt.Println("Available commands:")
	fmt.Println("\\me - Shows information about current account")
	fmt.Println("\\contacts - Shows contacts list")
	fmt.Println("\\umsg <id> <message> - Sends message to user with <id>")
	fmt.Println("\\cmsg <id> <message> - Sends message to chat with <id>")
	fmt.Println("\\pchannels - Shows information about current channels")
	fmt.Println("\\pac - Shows information about current user_channel_ac_hash")
	fmt.Println("\\invitechannel <channel_id> <user_id> - Invite user_id to channel_id")
	fmt.Println("\\help - Shows this message")
	fmt.Println("\\quit - Quit")
}

type TelegramCLI struct {
	mtproto   *mtproto.MTProto
	state     *mtproto.TL_updates_state
	connected bool
	users     map[int32]mtproto.TL_user
	chats     map[int32]mtproto.TL_chat
	channels  map[int32]mtproto.TL_channel
}

func NewTelegramCLI(pMTProto *mtproto.MTProto) (*TelegramCLI, error) {
	if pMTProto == nil {
		return nil, errors.New("NewTelegramCLI: pMTProto is nil")
	}
	cli := new(TelegramCLI)
	cli.mtproto = pMTProto
	cli.users = make(map[int32]mtproto.TL_user)
	cli.chats = make(map[int32]mtproto.TL_chat)
	cli.channels = make(map[int32]mtproto.TL_channel)

	return cli, nil
}

/////////////////// mything
func (cli *TelegramCLI) GetConfig() error {
	config, err := cli.mtproto.HelpGetConfig()
	if err != nil {
		return err
	}
	for idx := range config.Dc_options {
		tl := config.Dc_options[idx]
		fmt.Println(idx, "==--==")
		fmt.Println(tl.(mtproto.TL_dcOption))
	}
	fmt.Println(*config)
	return nil
}
func (cli *TelegramCLI) SearchContacts(q string, limit int) error {
	found, err := cli.mtproto.ContactsSearch(q, limit)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(*found)
	return nil
}
func (cli *TelegramCLI) Authorization(phonenumber string) error {
	if phonenumber == "" {
		return fmt.Errorf("Phone number is empty")
	}
	fmt.Println("i_0")
	sentCode, err := cli.mtproto.AuthSendCode(phonenumber)
	if err != nil {
		fmt.Println("AuthSendCode_err", err)
		return err
	}
	fmt.Println("i_.")
	if !sentCode.Phone_registered {
		return fmt.Errorf("Phone number isn't registered")
	}
	fmt.Println("i_..")
	var code string
	fmt.Printf("Enter code: ")
	fmt.Scanf("%s", &code)
	auth, err := cli.mtproto.AuthSignIn(phonenumber, code, sentCode.Phone_code_hash)
	if err != nil {
		return err
	}

	userSelf := auth.User.(mtproto.TL_user)
	cli.users[userSelf.Id] = userSelf
	message := fmt.Sprintf("Signed in: Id %d name <%s @%s %s>\n", userSelf.Id, userSelf.First_name, userSelf.Username, userSelf.Last_name)
	fmt.Print(message)
	log.Println(message)
	log.Println(userSelf)

	return nil
}

// Load contacts to users map
func (cli *TelegramCLI) LoadContacts() error {
	tl, err := cli.mtproto.ContactsGetContacts("")
	if err != nil {
		return err
	}
	list, ok := (*tl).(mtproto.TL_contacts_contacts)
	if !ok {
		return fmt.Errorf("RPC: %#v", tl)
	}

	for _, v := range list.Users {
		if v, ok := v.(mtproto.TL_user); ok {
			cli.users[v.Id] = v
		}
	}

	return nil
}

// Prints information about current user
func (cli *TelegramCLI) CurrentUser() error {
	userFull, err := cli.mtproto.UsersGetFullUsers(mtproto.TL_inputUserSelf{})
	if err != nil {
		return err
	}

	user := userFull.User.(mtproto.TL_user)
	cli.users[user.Id] = user

	message := fmt.Sprintf("You are logged in as: %s @%s %s\nId: %d\nPhone: %s\n", user.First_name, user.Username, user.Last_name, user.Id, user.Phone)
	fmt.Print(message)
	log.Println(message)
	log.Println(*userFull)

	return nil
}

// Connects to telegram server
func (cli *TelegramCLI) Connect() error {
	if err := cli.mtproto.Connect(); err != nil {
		return err
	}
	cli.connected = true
	log.Println("Connected to telegram server")
	return nil
}

// Disconnect from telegram server
func (cli *TelegramCLI) Disconnect() error {
	if err := cli.mtproto.Disconnect(); err != nil {
		return err
	}
	cli.connected = false
	log.Println("Disconnected from telegram server")
	return nil
}

// Run telegram cli
func (cli *TelegramCLI) Run() error {
	command := new(Command)
	command.Name = "iid"
	err := cli.RunCommand(command)
	if err != nil {
		log.Println(err)
	}
	return nil
}

// Parse message and print to screen
func (cli *TelegramCLI) parseMessage(message mtproto.TL) {
	switch message.(type) {
	case mtproto.TL_messageEmpty:
		log.Println("Empty message")
		log.Println(message)
	case mtproto.TL_message:
		log.Println("Got new message")
		log.Println(message)
		message, _ := message.(mtproto.TL_message)
		var senderName string
		from := message.From_id
		userFrom, found := cli.users[from]
		if !found {
			log.Printf("Can't find user with id: %d", from)
			senderName = fmt.Sprintf("%d unknow user", from)
		}
		senderName = nickname(userFrom)
		toPeer := message.To_id
		date := formatDate(message.Date)

		// Peer type
		switch toPeer.(type) {
		case mtproto.TL_peerUser:
			peerUser := toPeer.(mtproto.TL_peerUser)
			user, found := cli.users[peerUser.User_id]
			if !found {
				log.Printf("Can't find user with id: %d", peerUser.User_id)
				// TODO: Get information about user from telegram server
			}
			peerName := nickname(user)
			message := fmt.Sprintf("%s %d %s to %s: %s", date, message.Id, senderName, peerName, message.Message)
			fmt.Println(message)
		case mtproto.TL_peerChat:
			peerChat := toPeer.(mtproto.TL_peerChat)
			chat, found := cli.chats[peerChat.Chat_id]
			if !found {
				log.Printf("Can't find chat with id: %d", peerChat.Chat_id)
			}
			message := fmt.Sprintf("%s %d %s in %s(%d): %s", date, message.Id, senderName, chat.Title, chat.Id, message.Message)
			fmt.Println(message)
		case mtproto.TL_peerChannel:
			peerChannel := toPeer.(mtproto.TL_peerChannel)
			channel, found := cli.channels[peerChannel.Channel_id]
			if !found {
				log.Printf("Can't find channel with id: %d", peerChannel.Channel_id)
			}
			message := fmt.Sprintf("%s %d %s in %s(%d): %s", date, message.Id, senderName, channel.Title, channel.Id, message.Message)
			fmt.Println(message)
		default:
			log.Printf("Unknown peer type: %T", toPeer)
			log.Println(toPeer)
		}
	default:
		log.Printf("Unknown message type: %T", message)
		log.Println(message)
	}
}

// Works with mtproto.TL_updates_difference and mtproto.TL_updates_differenceSlice
func (cli *TelegramCLI) parseUpdateDifference(users, messages, chats, updates []mtproto.TL) {
	// Process users
	for _, it := range users {
		user, ok := it.(mtproto.TL_user)
		if !ok {
			log.Println("Wrong user type: %T\n", it)
		}
		cli.users[user.Id] = user
	}
	// Process chats
	for _, it := range chats {
		switch it.(type) {
		case mtproto.TL_channel:
			channel := it.(mtproto.TL_channel)
			cli.channels[channel.Id] = channel
		case mtproto.TL_chat:
			chat := it.(mtproto.TL_chat)
			cli.chats[chat.Id] = chat
		default:
			fmt.Printf("Wrong type: %T\n", it)
		}
	}
	// Process messages
	for _, message := range messages {
		cli.parseMessage(message)
	}
	// Process updates
	for _, it := range updates {
		switch it.(type) {
		case mtproto.TL_updateNewMessage:
			update := it.(mtproto.TL_updateNewMessage)
			cli.parseMessage(update.Message)
		case mtproto.TL_updateNewChannelMessage:
			update := it.(mtproto.TL_updateNewChannelMessage)
			cli.parseMessage(update.Message)
		case mtproto.TL_updateEditMessage:
			update := it.(mtproto.TL_updateEditMessage)
			cli.parseMessage(update.Message)
		case mtproto.TL_updateEditChannelMessage:
			update := it.(mtproto.TL_updateNewChannelMessage)
			cli.parseMessage(update.Message)
		default:
			log.Printf("Update type: %T\n", it)
			log.Println(it)
		}
	}
}

// Parse update
func (cli *TelegramCLI) parseUpdate(update mtproto.TL) {
	switch update.(type) {
	case mtproto.TL_updates_differenceEmpty:
		diff, _ := update.(mtproto.TL_updates_differenceEmpty)
		cli.state.Date = diff.Date
		cli.state.Seq = diff.Seq
	case mtproto.TL_updates_difference:
		diff, _ := update.(mtproto.TL_updates_difference)
		state, _ := diff.State.(mtproto.TL_updates_state)
		cli.state = &state
		cli.parseUpdateDifference(diff.Users, diff.New_messages, diff.Chats, diff.Other_updates)
	case mtproto.TL_updates_differenceSlice:
		diff, _ := update.(mtproto.TL_updates_differenceSlice)
		state, _ := diff.Intermediate_state.(mtproto.TL_updates_state)
		cli.state = &state
		cli.parseUpdateDifference(diff.Users, diff.New_messages, diff.Chats, diff.Other_updates)
	case mtproto.TL_updates_differenceTooLong:
		diff, _ := update.(mtproto.TL_updates_differenceTooLong)
		cli.state.Pts = diff.Pts
	}
}

// Get updates and prints result
func (cli *TelegramCLI) processUpdates() {
	if cli.connected {
		if cli.state == nil {
			log.Println("cli.state is nil. Trying to get actual state...")
			tl, err := cli.mtproto.UpdatesGetState()
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Got something")
			log.Println(*tl)
			state, ok := (*tl).(mtproto.TL_updates_state)
			if !ok {
				err := fmt.Errorf("Failed to get current state: API returns wrong type: %T", *tl)
				log.Fatal(err)
			}
			cli.state = &state
			return
		}
		tl, err := cli.mtproto.UpdatesGetDifference(cli.state.Pts, cli.state.Unread_count, cli.state.Date, cli.state.Qts)
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("Got new update")
		log.Println(*tl)
		cli.parseUpdate(*tl)
		return
	}
}

func (cli *TelegramCLI) Import_Invite_Delete() error {
	for idx := 0; idx != 10; idx++ {
		fmt.Println("-------")
		time.Sleep(time.Second * 1)
		cli.processUpdates()
		cli.PrintChannels("")
	}
	fmt.Println("channel_access_hash->", channel_access_hash)
	if len(channel_access_hash) == 0 {
		os.Exit(1)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		fmt.Println(text, "___", time.Now().Unix())
		cli.ImportContacts(text) // Import
		time.Sleep(time.Second * 1)
		var uid int32
		var uhash int64
		{
			// refresh my contacts
			cli.Contacts()
			time.Sleep(time.Second * 1)
			fmt.Println("user_hash->", user_achash)
			for _uid := range user_achash {
				uid = _uid
				uhash = user_achash[_uid]
			}
		}
		cli.InviteContactToChannel(fmt.Sprintf("1189158201 %v", uid)) // Invite
		time.Sleep(time.Second * 1)
		tl := new(mtproto.TL_inputUser) // delete
		{
			// userid and access_hash to compose tl
			tl.User_id = uid
			tl.Access_hash = uhash
		}
		cli.mtproto.DeleteContact(tl)

	}
	fmt.Println("all_over")
	return nil
}

// import contact
func (cli *TelegramCLI) ImportContacts(arg string) error {
	larens := []*mtproto.TL_inputPhoneContact{}
	onelaren := new(mtproto.TL_inputPhoneContact)
	onelaren.First_name = "golang_auto"
	onelaren.Last_name = fmt.Sprintf("%v", time.Now().Unix())
	onelaren.Phone = arg
	larens = append(larens, onelaren)
	cli.mtproto.ImportContacts(larens)
	return nil
}

var channel_access_hash map[int32]int64

// print channels
func (cli *TelegramCLI) PrintChannels(arg string) error {
	if channel_access_hash == nil {
		channel_access_hash = make(map[int32]int64)
	}
	for k, v := range cli.channels {
		fmt.Printf("%v-->[%v][%v]\n", k, v.Access_hash, v.Title)
		channel_access_hash[k] = v.Access_hash
	}
	return nil
}

// invite one contact to one channel
func (cli *TelegramCLI) InviteContactToChannel(arg string) error {
	time.Sleep(time.Second * 1)
	if arg == "" {
		return errors.New("no arg spec")
	}
	segs := strings.Split(arg, " ")
	i, _ := strconv.Atoi(segs[0]) // should be the channel id
	channelid := int32(i)
	var achash int64
	if as, found := channel_access_hash[channelid]; !found {
		return errors.New("wrong channel id")
	} else {
		achash = as
	}
	tl_channel := mtproto.TL_inputChannel{
		Channel_id:  channelid,
		Access_hash: achash,
	}
	// about the user
	i, _ = strconv.Atoi(segs[1]) // should be the user id
	userid := int32(i)
	var userachash int64
	if as, found := user_achash[userid]; !found {
		return errors.New("wrong user id")
	} else {
		userachash = as
	}
	tl_users := []mtproto.TL{}
	ele_user := mtproto.TL_inputUser{
		User_id:     userid,
		Access_hash: userachash,
	}
	tl_users = append(tl_users, ele_user)
	cli.mtproto.InviteToChannel(tl_users, tl_channel)
	return nil
}

var user_achash map[int32]int64

// Print contact list
func (cli *TelegramCLI) Contacts() error {
	user_achash = make(map[int32]int64)
	tl, err := cli.mtproto.ContactsGetContacts("")
	if err != nil {
		return err
	}
	list, ok := (*tl).(mtproto.TL_contacts_contacts)
	if !ok {
		return fmt.Errorf("RPC: %#v", tl)
	}

	contacts := make(map[int32]mtproto.TL_user)
	for _, v := range list.Users {
		if v, ok := v.(mtproto.TL_user); ok {
			contacts[v.Id] = v
		}
	}
	fmt.Printf(
		"\033[33m\033[1m%10s    %10s    %-30s    %-20s\033[0m\n",
		"id", "mutual", "name", "username",
	)
	for _, v := range list.Contacts {
		v := v.(mtproto.TL_contact)
		mutual, err := mtproto.ToBool(v.Mutual)
		if err != nil {
			return err
		}
		fmt.Printf(
			"%10d    %10t    %-30s    %-20s  %-20d\n",
			v.User_id,
			mutual,
			fmt.Sprintf("%s %s", contacts[v.User_id].First_name, contacts[v.User_id].Last_name),
			contacts[v.User_id].Username, contacts[v.User_id].Access_hash,
		)

		if contacts[v.User_id].First_name == "golang_auto" {
			fmt.Println("golang_auto is in")
			user_achash[v.User_id] = contacts[v.User_id].Access_hash
		} else {
			fmt.Println("golang_auto is NOT")
		}
	}

	return nil
}

// Runs command and prints result to console
func (cli *TelegramCLI) RunCommand(command *Command) error {
	switch command.Name {
	case "iid":
		cli.Import_Invite_Delete()
	case "me":
		if err := cli.CurrentUser(); err != nil {
			return err
		}
	case "contacts":
		if err := cli.Contacts(); err != nil {
			return err
		}
	case "high_invite":
		fmt.Println(command.Arguments)
	case "umsg":
		if command.Arguments == "" {
			return errors.New("Not enough arguments: peer id and msg required")
		}
		args := strings.SplitN(command.Arguments, " ", 2)
		if len(args) < 2 {
			return errors.New("Not enough arguments: peer id and msg required")
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("Wrong arguments: %s isn't a number", args[0])
		}
		user, found := cli.users[int32(id)]
		if !found {
			info := fmt.Sprintf("Can't find user with id: %d", id)
			fmt.Println(info)
			return nil
		}
		update, err := cli.mtproto.MessagesSendMessage(false, false, false, true, mtproto.TL_inputPeerUser{User_id: user.Id, Access_hash: user.Access_hash}, 0, args[1], rand.Int63(), mtproto.TL_null{}, nil)
		cli.parseUpdate(*update)
	case "pchannels":
		fmt.Println(command.Arguments)
		cli.PrintChannels(command.Arguments)
	case "pac":
		fmt.Println("user_access_hash", user_achash)
		fmt.Println("channel_access_hash", channel_access_hash)
	case "invitechannel":
		err := cli.InviteContactToChannel(command.Arguments)
		if err != nil {
			fmt.Println(err)
		}
	case "cmsg":
		if command.Arguments == "" {
			return errors.New("Not enough arguments: peer id and msg required")
		}
		args := strings.SplitN(command.Arguments, " ", 2)
		if len(args) < 2 {
			return errors.New("Not enough arguments: peer id and msg required")
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("Wrong arguments: %s isn't a number", args[0])
		}
		update, err := cli.mtproto.MessagesSendMessage(false, false, false, true, mtproto.TL_inputPeerChat{Chat_id: int32(id)}, 0, args[1], rand.Int63(), mtproto.TL_null{}, nil)
		cli.parseUpdate(*update)
	case "search":

	case "help":
		help()
	case "quit":
		cli.Disconnect()
	default:
		fmt.Println("Unknow command. Try \\help to see all commands")
		return errors.New("Unknow command")
	}
	return nil
}

func main() {
	logfile, err := os.OpenFile("logfile.txt", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logfile.Close()

	log.SetOutput(logfile)
	log.Println("Program started")

	// LoadContacts
	mtproto, err := mtproto.NewMTProto(400495, "180b6f8d2cc00beb4dbca0500416a41f", mtproto.WithAuthFile(os.Getenv("HOME")+"/.telegramgo", false))
	if err != nil {
		log.Fatal(err)
	}
	telegramCLI, err := NewTelegramCLI(mtproto)
	if err != nil {
		log.Fatal(err)
	}
	if err = telegramCLI.Connect(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Welcome to telegram CLI")
	{
		if err := telegramCLI.GetConfig(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		} else {
			fmt.Println("success_get_config")
		}
	}
	if err := telegramCLI.CurrentUser(); err != nil {
		var phonenumber string
		fmt.Println("Enter phonenumber number below: ")
		fmt.Scanln(&phonenumber)
		fmt.Println(phonenumber)
		err := telegramCLI.Authorization(phonenumber)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println(".")
	if err := telegramCLI.LoadContacts(); err != nil {
		log.Fatalf("Failed to load contacts: %s", err)
	}
	fmt.Println("..")

	// Show help first time
	help()
	fmt.Println("...")

	err = telegramCLI.Run()
	if err != nil {
		log.Println(err)
		fmt.Println("Telegram CLI exits with error: ", err)
	}

}
