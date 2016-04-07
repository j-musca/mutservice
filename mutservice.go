package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/buaazp/fasthttprouter"
	"github.com/nu7hatch/gouuid"
	"github.com/valyala/fasthttp"
	"log"
	"os"
	"time"
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"io/ioutil"
	"github.com/ddliu/go-httpclient"
	"github.com/robfig/cron"
)

var USERS_BUCKET []byte = []byte("users")
var KEYS_BUCKET []byte = []byte("keys")
var MOODS_BUCKET []byte = []byte("moods")
var BUCKET_NAMES [][]byte = [][]byte{USERS_BUCKET, KEYS_BUCKET, MOODS_BUCKET}

func main() {
	dataBase, dataBaseError := bolt.Open(os.Getenv("HOME")+"/app-mut.db", 0600, &bolt.Options{Timeout: 1 * time.Second})

	if dataBaseError != nil {
		log.Fatal(dataBaseError)
	}

	defer dataBase.Close()

	scheduler := cron.New()

	scheduler.AddFunc("14 30 * * * *", triggerMail(dataBase))

	dataBase.Update(createBuckets)

	serverError := fasthttp.ListenAndServe(":8081", createRouter(dataBase).Handler)

	if serverError != nil {
		log.Fatalf("Error in ListenAndServe: %s", serverError)
	}
}

func sendMail(email string, text string) {
	response, responseError := httpclient.
	WithHeader("Authorization", "Basic YXBpOmtleS00NWY0ODYxNTVkZmUzZjUxY2ExOTg4MjEwNGIzYmViMg==").
	Post("https://api.mailgun.net/v3/sandbox4ebeef9e81ca4130885ef51fa4b9729f.mailgun.org/messages", map[string]string {
		"from": "Mailgun Sandbox <postmaster@sandbox4ebeef9e81ca4130885ef51fa4b9729f.mailgun.org>",
		"to": email,
		"subject": "How is your mood today?",
		"text": text,
	})

	if responseError != nil {
		fmt.Printf("%s", responseError)
	}

	defer response.Body.Close()

	_, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("%s", err)
	}
}

func triggerMail(db *bolt.DB) {
	return func() {
		users, triggerError := getUsers(db)

		if triggerError != nil {
			fmt.Printf("%s", triggerError)
		}

		mailTasks, triggerError := createMailTasks(users, db)

		if triggerError != nil {
			fmt.Printf("%s", triggerError)
		}

		sendMails(mailTasks)
	}
}

func sendMails(tasks []MailTask) {
	for _, task := range tasks {
		sendMail(task.Email, "" + task.Key)
	}
}

func createMailTasks(users []User, db *bolt.DB) (tasks []MailTask, dbError error) {
	today := time.Now().Format("02-01-2006")

	dbError = db.Update(func(tx *bolt.Tx) error {
		tx.DeleteBucket(KEYS_BUCKET)
		bucket, _ := tx.CreateBucket(KEYS_BUCKET)

		for _, user := range users {
			key := createKey(user.Uuid, today)

			bucketError := bucket.Put([]byte(key), []byte(today))

			if bucketError != nil {
				return bucketError
			}

			tasks = append(tasks, MailTask{user.Email, key})
		}

		return nil
	})

	return tasks, dbError
}

func createKey(uuid string, dateString string) (key string) {
	source := strings.Join([]string{uuid, dateString}, "-")
	hashCreator := sha1.New()
	hashCreator.Write([]byte(source))
	key = hex.EncodeToString(hashCreator.Sum(nil))

	return key
}

func createBuckets(tx *bolt.Tx) error {
	for _, element := range BUCKET_NAMES {
		_, err := tx.CreateBucketIfNotExists(element)

		if err != nil {
			return fmt.Errorf("Could not create bucket: %s", err)
		}
	}

	return nil
}

func createRouter(db *bolt.DB) (router *fasthttprouter.Router) {
	router = fasthttprouter.New()

	router.GET("/users", getUsersRequest(db))
	router.GET("/users/:uuid", getUserByUuidRequest(db))
	router.POST("/users", putUserRequest(db))

	return router
}

func getUsersRequest(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		users, dbError := getUsers(db)

		if dbError != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBody([]byte(dbError.Error()))
		} else {
			usersResponse := UsersResponse{users}
			json, jsonError := json.Marshal(usersResponse)

			if jsonError != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBody([]byte(jsonError.Error()))
			} else {
				ctx.SetStatusCode(fasthttp.StatusOK)
				ctx.SetContentType("application/json")
				ctx.SetBody(json)
			}

		}
	})
}

func getUsers(db *bolt.DB) (users []User, dbError error) {
	dbError = db.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(USERS_BUCKET)
		usersBucket.ForEach(func(uuid []byte, email []byte) error {
			users = append(users, User{string(uuid), string(email)})
			return nil
		})
		return nil
	})

	return users, dbError
}

func getUserByUuidRequest(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		uuid := params.ByName("uuid")
		user, dbError := getUserByUuid(db, uuid)

		if dbError != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBody([]byte(dbError.Error()))
		} else {
			if user != nil {
				json, jsonError := json.Marshal(user)

				if jsonError != nil {
					ctx.SetStatusCode(fasthttp.StatusInternalServerError)
					ctx.SetBody([]byte(jsonError.Error()))
				} else {
					ctx.SetStatusCode(fasthttp.StatusOK)
					ctx.SetContentType("application/json")
					ctx.SetBody(json)
				}
			} else {
				ctx.SetStatusCode(fasthttp.StatusNotFound)
				ctx.SetBody([]byte("{\"message\":\"User with uuid '" + uuid + "' not found!\"}"))
			}
		}
	})
}

func getUserByUuid(db *bolt.DB, uuid string) (user *User, dbError error) {
	dbError = db.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(USERS_BUCKET)
		email := usersBucket.Get([]byte(uuid))

		if email != nil {
			user = &User{uuid, string(email)}
		}

		return nil
	})

	return user, dbError
}

func putUserRequest(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		userCreation := UserCreation{}
		jsonError := json.Unmarshal(ctx.PostBody(), &userCreation)

		if jsonError != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBody([]byte(jsonError.Error()))
		} else {
			user, dbError := saveUser(db, userCreation)

			fmt.Printf("Saved user: %s\n", user)

			if dbError != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBody([]byte(dbError.Error()))
			} else {
				json, _ := json.Marshal(user)
				ctx.SetStatusCode(fasthttp.StatusCreated)
				ctx.SetContentType("application/json")
				ctx.SetBody(json)
			}

		}

	})
}

func saveUser(db *bolt.DB, userCreation UserCreation) (user User, err error) {
	uuid, _ := uuid.NewV4()

	dbError := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(USERS_BUCKET)
		return bucket.Put([]byte(uuid.String()), []byte(userCreation.Email))
	})

	return User{uuid.String(), userCreation.Email}, dbError
}

type FeedbackKey struct {
	Key string
	DateString string
}

type User struct {
	Uuid  string `json:"uuid"`
	Email string `json:"email"`
}

type UsersResponse struct {
	Users []User `json:"users"`
}

type UserCreation struct {
	Email string `json:"email"`
}

type MailTask struct {
	Email string
	Key string
}
