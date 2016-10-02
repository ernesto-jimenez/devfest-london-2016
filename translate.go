package translate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/translate/v2"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

func init() {
	// Register a handler on the `/command` which checks the Slack token
	http.HandleFunc("/command", checkToken(os.Getenv("TOKEN"), command))
}

// checkToken middleware returns a handler that ensures the token is valid
func checkToken(token string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// context is used to link logs and requests with the current request
		ctx := appengine.NewContext(r)

		// respond with an error when the token is invalid
		if r.PostFormValue("token") != token {
			log.Warningf(ctx, "Invalid token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// call the wrapped handler if the token is valid
		h(w, r)
	}
}

// apiKey contains the google translate API Key
var apiKey = os.Getenv("TRANSLATE_KEY")

type message struct {
	Text         string       `json:"text"`
	Attachments  []attachment `json:"attachments,omitempty"`
	ResponseType string       `json:"response_type,omitempty"`
}

type attachment struct {
	Text  string `json:"text"`
	Color string `json:"color"`
}

func command(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	// Initialise the translate service
	ts, err := translate.New(&http.Client{
		// Use google's Transport with the translate key
		Transport: &transport.APIKey{
			Key: apiKey,
			// Google App Engine does not give direct access to the internet
			// we need to use `urlfetch` to create an App Engine transport
			Transport: &urlfetch.Transport{Context: ctx},
		},
	})
	if err != nil {
		log.Errorf(ctx, "Error initialising translate service: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Translate the command's text into english
	t := ts.Translations.List([]string{r.PostFormValue("text")}, "en")

	// Call the Google Translate API
	res, err := t.Do()
	if err != nil {
		log.Errorf(ctx, "Error translating: %s", err.Error())
		// Respond with a message only the person who called the command can see
		json.NewEncoder(w).Encode(message{
			Text: "Error",
			Attachments: []attachment{
				{Color: "danger", Text: err.Error()},
			},
		})
		return
	}

	// Respond with a message everybody in the channel can see
	msg := message{
		Text:         fmt.Sprintf("Translating %q", r.PostFormValue("text")),
		ResponseType: "in_channel",
	}

	for _, t := range res.Translations {
		msg.Attachments = append(msg.Attachments, attachment{
			Color: "good",
			Text: fmt.Sprintf(
				"%s (%s)", t.TranslatedText, t.DetectedSourceLanguage,
			),
		})
	}

	json.NewEncoder(w).Encode(msg)
}
