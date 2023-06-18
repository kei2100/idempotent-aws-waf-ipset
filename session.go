package ipset

import (
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
)

// Session is an AWS session
var Session *session.Session

func init() {
	var err error
	Session, err = session.NewSessionWithOptions(
		session.Options{
			SharedConfigState: session.SharedConfigEnable,
		},
	)
	if err != nil {
		log.Fatalf("main: new aws session: %+v", err)
	}
}
