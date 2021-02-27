package web

import (
	"encoding/json"
	"fmt"
	"github.com/gme-sh/gme.sh-api/pkg/gme-sh/short"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

// CreateResponseOK is returned when everything worked fine
const CreateResponseOK = "success"

type createShortURLPayload struct {
	FullURL            string `json:"full_url"`
	PreferredAlias     string `json:"preferred_alias"`
	ExpireAfterSeconds int    `json:"expire_after_seconds"`
}

type createShortURLResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Short   *short.ShortURL `json:"short"`
}

func dieCreate(w http.ResponseWriter, o interface{}) {
	var res *createShortURLResponse

	switch v := o.(type) {
	case error:
		res = &createShortURLResponse{
			Success: false,
			Message: v.Error(),
			Short:   nil,
		}
		break
	case string:
		res = &createShortURLResponse{
			Success: false,
			Message: v,
			Short:   nil,
		}
		break
	case *createShortURLResponse:
		res = v
		break
	}

	if res == nil {
		res = &createShortURLResponse{
			Success: false,
			Message: "an unknown error occurred.",
			Short:   nil,
		}
	}

	var msg []byte
	var err error
	msg, err = json.Marshal(res)
	if err != nil {
		_, _ = fmt.Fprintln(w, res.Message)
		return
	}

	if res.Success {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(400)
	}

	_, _ = fmt.Fprintln(w, string(msg))
}

// mux
// POST application/json /api/v1/create
func (ws *WebServer) handleApiV1Create(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	log.Println("🚀", r.RemoteAddr, "requested to POST create a new short URL")

	// read body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("    └ 🤬 But the body was weird (read)")
		dieCreate(w, err)
		return
	}

	// parse body
	var req *createShortURLPayload
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Println("    └ 🤬 But the body was weird (json)")
		dieCreate(w, err)
		return
	}

	// Check blocks / loops
	u, err := url.Parse(req.FullURL)
	if err != nil {
		dieCreate(w, err)
		return
	}
	if err := ws.checkDomain(u); err != nil {
		dieCreate(w, err)
		return
	}
	//

	checkedAvailability := false

	/// Generate ID and check if alias already exists
	if req.PreferredAlias == "" {
		if generated := short.GenerateShortID(ws.PersistentDatabase.ShortURLAvailable); generated != "" {
			req.PreferredAlias = generated.String()
			checkedAvailability = true
		} else {
			log.Println("    └ 🤬 But all tried aliases were already occupied")
			dieCreate(w, "generated id not available")
			return
		}
	}
	log.Println("    └ 👉 Preferred alias:", req.PreferredAlias)

	// ShortURL Info
	aliasID := short.ShortID(req.PreferredAlias)
	duration := time.Duration(req.ExpireAfterSeconds) * time.Second

	// check if alias already exists
	if !checkedAvailability {
		if available := ws.PersistentDatabase.ShortURLAvailable(&aliasID); !available {
			log.Println("    └ 🤬 But the preferred was already occupied")
			dieCreate(w, "preferred alias is not available")
			return
		}
	}
	///

	var expiration *time.Time
	if duration > 0 {
		v := time.Now().Add(duration)
		expiration = &v
	}

	// create short id
	secret := short.GenerateID(32, short.AlwaysTrue, 0)
	sh := &short.ShortURL{
		ID:             short.ShortID(req.PreferredAlias),
		FullURL:        req.FullURL,
		CreationDate:   time.Now(),
		ExpirationDate: expiration,
		Secret:         secret.String(),
	}

	if err := ws.PersistentDatabase.SaveShortenedURL(sh); err != nil {
		log.Println("    └ 🤬 But something went wrong saving (persistent):", err)
		dieCreate(w, err)
		return
	}

	log.Println("    └ 💚 Looks like it worked out")
	dieCreate(w, &createShortURLResponse{
		Success: true,
		Message: CreateResponseOK,
		Short:   sh,
	})
}
