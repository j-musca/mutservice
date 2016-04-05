package main


import (
	"log"
	"os"
	"github.com/valyala/fasthttp"
	"github.com/buaazp/fasthttprouter"
	"github.com/boltdb/bolt"
	"time"
	"fmt"
	"encoding/json"
	"github.com/nu7hatch/gouuid"
)

func main() {
	db, boltErr := bolt.Open(os.Getenv("HOME") + "/app-mut.db", 0600, &bolt.Options{Timeout: 1 * time.Second})

	if boltErr != nil {
		log.Fatal(boltErr)
	}

	defer db.Close()

	db.Update(createBuckets)

	serverError := fasthttp.ListenAndServe(":8080", createRouter(db).Handler)

	if serverError != nil {
		log.Fatalf("Error in ListenAndServe: %s", serverError)
	}
}

func createBuckets(tx *bolt.Tx) error {
	bucketNames := [3]string{"users", "keys", "moods"}

	for _, element := range bucketNames {
		_, err := tx.CreateBucketIfNotExists([]byte(element))

		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
	}

	return nil
}

func createRouter(db *bolt.DB) (router *fasthttprouter.Router) {
	router = fasthttprouter.New()

	router.GET("/users", getUsers(db))
	//router.GET("/users/:uuid", getUserById(db))
	router.POST("/users", saveUser(db))

	return router
}

func getUsers(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		users, dbError := getUsersFromDB(db)

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

func getUsersFromDB(db *bolt.DB) (users []User, dbError error) {
	fmt.Println("Load Users!")
	dbError = db.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte("users"))
		usersBucket.ForEach(func(uuid []byte, email []byte) error {
			users = append(users, User{string(uuid), string(email)})
			return nil
		})
		return nil
	})

	return users, dbError
}

func saveUser(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		var userCreation UserCreation

		fmt.Println(string(ctx.PostBody()))

		jsonError := json.Unmarshal(ctx.PostBody(), &userCreation)
		fmt.Println(userCreation.email)
		if jsonError != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBody([]byte(jsonError.Error()))
		} else {
			user, dbError := insertUserToDB(db, userCreation)

			fmt.Println(user)

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

func insertUserToDB(db *bolt.DB, userCreation UserCreation) (user User, err error) {
	uuid, _ := uuid.NewV4()

	fmt.Println("Saved User!")
	dbError := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))
		return bucket.Put([]byte(uuid.String()), []byte(userCreation.email))
	})

	return User{uuid.String(), userCreation.email}, dbError
}

type User struct {
	uuid string
	email string
}

type UsersResponse struct {
	users []User
}

type UserCreation struct {
	email string
}