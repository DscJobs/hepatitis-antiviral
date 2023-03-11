package main

import (
	"context"
	"errors"
	"fmt"
	"hepatitis-antiviral/cli"
	"hepatitis-antiviral/migrations"
	"hepatitis-antiviral/sources/mongo"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/infinitybotlist/eureka/crypto"
	"go.mongodb.org/mongo-driver/bson"
)

var sess *discordgo.Session

var ctx = context.Background()

// Schemas here
//
// Either use schema struct tag (or bson + mark struct tag for special type overrides)

// Tool below

var source mongo.MongoSource

type UUID = string

type Details struct {
	UserID     string `src:"userID" dest:"userID" unique:"true" fkey:"users,userID"`
	ExpServers int    `src:"exp_servers" dest:"expServers"`
	Exp        int    `src:"exp" dest:"exp"`
	Active     int    `src:"active" dest:"active"`
	Salary     int    `src:"salary" dest:"salary"`
}

type CV struct {
	UserID    string    `src:"userID" dest:"userID" unique:"true" fkey:"users,userID"`
	Overview  string    `src:"overview" dest:"overview"`
	Hire      string    `src:"hire" dest:"hire"`
	Birthday  time.Time `src:"ddhhd" dest:"birthday" default:"NOW()"`
	Link      string    `src:"link,omitempty" dest:"link" default:"null"`
	Email     string    `src:"email,omitempty" dest:"email" default:"null"`
	Job       string    `src:"job,omitempty" dest:"job" default:"null"`
	Vanity    string    `src:"vanity,omitempty" dest:"vanity" default:"null"`
	Private   bool      `src:"private" dest:"private" default:"false"`
	Developer bool      `src:"developer" dest:"developer" default:"false"`
	Current   bool      `src:"current" dest:"current" default:"false"`
	ExpToggle bool      `src:"expToggle" dest:"expToggle" default:"false"`
	Nitro     bool      `src:"nitro" dest:"nitro" default:"false"`
	Views     int       `src:"views" dest:"views" default:"0"`
	Likes     int       `src:"likes" dest:"likes" default:"0"`
	Date      time.Time `src:"shhs" dest:"date" default:"NOW()"`
}

type Requests struct {
	UserID  string `src:"userID" dest:"userID" fkey:"users,userID"`
	CV      string `src:"cv" dest:"cv"`
	Content string `src:"content" dest:"content"`
	Tag     string `src:"tag" dest:"tag"`
}

type Review struct {
	UserID  string    `src:"userID" dest:"userID" fkey:"users,userID"`
	CV      string    `src:"cv" dest:"cv"`
	Content string    `src:"content" dest:"content"`
	Likes   []string  `src:"likes" dest:"likes" default:"{}"`
	Reports []string  `src:"reports" dest:"reports" default:"{}"`
	Type    string    `src:"type" dest:"type" default:"pending"`
	Rate    int       `src:"rate" dest:"rate" default:"0"`
	Date    time.Time `src:"date" dest:"date" default:"now()"`
}

type User struct {
	UserID          string            `src:"userID" dest:"userID" unique:"true"`
	Token           string            `src:"token" dest:"token"`
	Votes           []string          `src:"votes" dest:"votes" default:"{}"`
	Banned          bool              `src:"banned" dest:"banned" default:"false"`
	Staff           bool              `src:"staff" dest:"staff" default:"false"`
	Premium         bool              `src:"premium" dest:"premium" default:"false"`
	LifetimePremium bool              `src:"lifetime" dest:"lifetime_premium" default:"false"`
	PremiumDuration time.Time         `src:"duration" dest:"premiumDuration" default:"now()"`
	Notifications   map[string]string `src:"notifications" dest:"notifications" default:"{}"`
}

var userTransforms = map[string]cli.TransformFunc{
	"Token": func(a cli.TransformRow) any {
		return crypto.RandString(255)
	},
}

var botTransforms = map[string]cli.TransformFunc{}

func main() {
	// Place all schemas to be used in the tool here

	cli.Main(cli.App{
		SchemaOpts: cli.SchemaOpts{
			TableName: "dscjobs",
		},
		// Required
		LoadSource: func(name string) (cli.Source, error) {
			switch name {
			case "mongo":
				source = mongo.MongoSource{
					ConnectionURL:  os.Getenv("MONGO"),
					DatabaseName:   "dscjobs",
					IgnoreEntities: []string{},
				}

				err := source.Connect()

				if err != nil {
					return nil, err
				}

				return source, nil
			}

			return nil, errors.New("unknown source")
		},
		BackupFunc: func(src cli.Source) {
			var err error
			sess, err = discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))

			if err != nil {
				panic(err)
			}

			sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

			err = sess.Open()

			if err != nil {
				panic(err)
			}

			cli.BackupTool(source, "members", User{}, cli.BackupOpts{
				RenameTo:   "users",
				IndexCols:  []string{},
				Transforms: userTransforms,
			})

			cli.BackupTool(source, "users", CV{}, cli.BackupOpts{
				RenameTo:   "cv",
				IndexCols:  []string{},
				Transforms: botTransforms,
			})

			cli.BackupTool(source, "details", Details{}, cli.BackupOpts{
				IndexCols:  []string{},
				Transforms: botTransforms,
			})

			cli.BackupTool(source, "requests", Requests{}, cli.BackupOpts{
				IndexCols:  []string{},
				Transforms: botTransforms,
			})

			migrations.Migrate(context.Background(), cli.Pool)

			// Handle details from old
			col := source.Database.Collection("users")

			cur, err := col.Find(ctx, bson.M{})

			if err != nil {
				panic(err)
			}

			for cur.Next(ctx) {
				var data struct {
					UserID  string `bson:"userID"`
					Details []struct {
						Exp        any `bson:"exp"`
						ExpServers any `bson:"exp_servers"`
						Active     any `bson:"active"`
						Salary     any `bson:"salary"`
					} `bson:"details"`
				}

				err := cur.Decode(&data)

				if err != nil {
					fmt.Println(data)
					panic(err)
				}

				_, err = cli.Pool.Exec(context.Background(), "INSERT INTO details (userID, exp, expServers, active, salary) VALUES ($1, $2, $3, $4, $5)", data.UserID, data.Details[0].Exp, data.Details[0].ExpServers, data.Details[0].Active, data.Details[0].Salary)

				if err != nil {
					panic(err)
				}
			}
		},
	})
}
