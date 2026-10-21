package request

import (
	"context"
	"testing"
)

func TestUserFrom(t *testing.T) {
	if _, ok := UserFrom(context.Background()); ok {
		t.Fatal("empty context must have no user")
	}
	if _, ok := UserFrom(WithUser(context.Background(), User{})); ok {
		t.Fatal("zero id must count as signed out")
	}
	u, ok := UserFrom(WithUser(context.Background(), User{ID: 7}))
	if !ok || u.ID != 7 {
		t.Fatalf("got %+v, %v", u, ok)
	}
}
