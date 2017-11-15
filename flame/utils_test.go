package flame

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/256dpi/fire/coal"

	"golang.org/x/crypto/bcrypt"
)

var testStore = coal.MustCreateStore("mongodb://0.0.0.0:27017/test-flame")
var testSubStore = testStore.Copy()

func cleanSubStore() {
	testSubStore.DB().C("users").RemoveAll(nil)
	testSubStore.DB().C("applications").RemoveAll(nil)
	testSubStore.DB().C("access_tokens").RemoveAll(nil)
	testSubStore.DB().C("refresh_tokens").RemoveAll(nil)
}

func newHandler(auth *Authenticator, force bool) http.Handler {
	router := http.NewServeMux()

	router.Handle("/oauth2/", auth.Endpoint("/oauth2/"))

	authorizer := auth.Authorizer("foo", force)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	router.Handle("/api/protected", authorizer(handler))

	return router
}

func saveModel(m coal.Model) coal.Model {
	err := testSubStore.C(m).Insert(m)
	if err != nil {
		panic(err)
	}

	return m
}

func mustHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}

	return hash
}

func testRequest(h http.Handler, method, path string, headers map[string]string, payload string, callback func(*httptest.ResponseRecorder, *http.Request)) {
	r, err := http.NewRequest(method, path, strings.NewReader(payload))
	if err != nil {
		panic(err)
	}

	w := httptest.NewRecorder()

	for k, v := range headers {
		r.Header.Set(k, v)
	}

	h.ServeHTTP(w, r)

	callback(w, r)
}