package main

import (
	"bufio"
	"context"
	"fmt"
	stt "github.com/LazarenkoA/SpeechToTxt/STT"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"golang.org/x/xerrors"
	"net/http"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	key       = "" // статический ключ доступа
	apikey    = "" // API key
	ID_apikey = ""
	bucket    = ""
)

type telegramWrap struct {
	raw    *tg.Client
	client *telegram.Client
	ctx    context.Context

	// получаются тут https://my.telegram.org/auth
	AppID   int
	AppHash string

	myID int
}

func (t *telegramWrap) newClient() error {
	// для yandex stt
	key = os.Getenv("KEY")
	apikey = os.Getenv("APIKEY")
	bucket = os.Getenv("BUCKET")
	ID_apikey = os.Getenv("IDAPIKEY")

	// для клиента телеги
	t.AppID, _ = strconv.Atoi(os.Getenv("APPID"))
	t.AppHash = os.Getenv("APPHASH")

	if err := t.check(); err != nil {
		return err
	}

	t.ctx = context.Background()

	// Setting up session storage.
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	sessionDir := filepath.Join(dir, ".td")
	if err := os.MkdirAll(sessionDir, 0600); err != nil {
		return err
	}

	var SessionStorage telegram.SessionStorage
	if sessiondata := os.Getenv("SESSIONDATA"); sessiondata != "" {
		SessionStorage = new(session.StorageMemory)
		SessionStorage.StoreSession(t.ctx, []byte(sessiondata))
	} else {
		SessionStorage = &telegram.FileSessionStorage{
			Path: filepath.Join(sessionDir, "session.json"),
		}
	}

	dispatcher := tg.NewUpdateDispatcher()
	t.client = telegram.NewClient(t.AppID, t.AppHash, telegram.Options{
		SessionStorage: SessionStorage,
		UpdateHandler:  dispatcher,
	})

	t.setDispatcher(&dispatcher)

	return nil
}

func (t *telegramWrap) Run(f func()) {
	// ****************** нужно для хероку ******************
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	go http.ListenAndServe(":"+port, nil)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "working")
	})
	// ******************

	err := t.client.Run(t.ctx, func(ctx context.Context) error {
		auth_ := t.client.Auth()
		authStatus, err := auth_.Status(ctx)

		if err != nil || !authStatus.Authorized {
			var phonenumber string
			fmt.Println("Введите номер телефона: ")
			fmt.Scanln(&phonenumber)

			codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
				// NB: Use "golang.org/x/crypto/ssh/terminal" to prompt password.
				fmt.Print("Введите код: ")
				code, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(code), nil
			}

			if err := auth.NewFlow(
				auth.Constant(phonenumber, "", auth.CodeAuthenticatorFunc(codePrompt)),
				auth.SendCodeOptions{},
			).Run(ctx, auth_); err != nil {
				return err
			}
		}

		// Using tg.Client for directly calling RPC.
		t.raw = tg.NewClient(t.client)
		_, err = t.raw.UpdatesGetState(ctx)
		if err != nil {
			return xerrors.Errorf("не удалось получить состояние: %w", err)
		}
		f()

		<-ctx.Done()
		return ctx.Err()
	})

	if err != nil {
		fmt.Println(err)
	}
}

func (t *telegramWrap) check() error {
	ers := []string{}
	if key == "" {
		ers = append(ers, "в переменных окружения не заполнен KEY")
	}
	if apikey == "" {
		ers = append(ers, "в переменных окружения не заполнен APIKEY")
	}
	if bucket == "" {
		ers = append(ers, "в переменных окружения не заполнен BUCKET")
	}
	if ID_apikey == "" {
		ers = append(ers, "в переменных окружения не заполнен IDAPIKEY")
	}
	if t.AppID == 0 {
		ers = append(ers, "в переменных окружения не заполнен APPID")
	}
	if t.AppHash == "" {
		ers = append(ers, "в переменных окружения не заполнен APPHASH")
	}
	if len(ers) > 0 {
		return fmt.Errorf("не все обязательные параметры заполнены:\n%s", strings.Join(ers, "\n"))
	} else {
		return nil
	}
}

func (t *telegramWrap) setDispatcher(dispatcher *tg.UpdateDispatcher) {
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		if msg, ok := update.Message.(*tg.Message); ok {
			if filepath := t.downloadAudioMessage(ctx, msg); filepath != "" {
				defer os.Remove(filepath)
				txt := t.SpeechToTxt(filepath)

				for id, _ := range e.Users {
					if id != t.myID && txt != "" {
						t.sendMsg(id, txt, msg.ID)
					}
				}
			}
		}
		return nil
	})
}

func (t *telegramWrap) downloadAudioMessage(ctx context.Context, msg *tg.Message) string {
	tmpfile, _ := os.CreateTemp("", "*.ogg")
	tmpfile.Close()

	mediaDoc, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return ""
	}
	abstractdoc, ok := mediaDoc.GetDocument()
	if !ok {
		return ""
	}
	if doc, ok := abstractdoc.(*tg.Document); ok {
		if doc.MimeType != message.DefaultVoiceMIME {
			return ""
		}

		downloader.NewDownloader().Download(t.raw, &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
			ThumbSize:     "",
		}).ToPath(ctx, tmpfile.Name())

		return tmpfile.Name()
	}

	return ""
}

func (t *telegramWrap) SpeechToTxt(filepath string) string {
	// репа приватная по этому в коде
	sttObj := new(stt.STT).New(&stt.STTConf{
		Key:       key,
		ID_apikey: ID_apikey,
		Apikey:    apikey,
		Bucket:    bucket,
	})

	out := make(chan string, 1)
	if err := sttObj.UploadStorageYandexcloud(filepath); err == nil {
		if err = sttObj.SpeechKit(out); err != nil {
			close(out)
			fmt.Println("Ошибка распознования", err)
		}
	} else {
		fmt.Println("Ошибка заргузки файла в облако ", err)
		close(out)
	}

	return <-out
}

func (t *telegramWrap) sendMsg(ID int, txt string, pMsgID int) {
	peer := &tg.InputPeerUser{
		UserID: ID,
		//AccessHash: user.AccessHash,

	}

	sender := message.NewSender(t.raw)
	_, err := sender.To(peer).Reply(pMsgID).Text(t.ctx, txt)

	if err != nil {
		fmt.Printf("произошла ошибка при отправке сообщения %v\n", err)
	}
}
