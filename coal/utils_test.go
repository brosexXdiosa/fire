package coal

import (
	"errors"
	"time"

	"gopkg.in/mgo.v2/bson"
)

type postModel struct {
	Base       `json:"-" bson:",inline" coal:"posts"`
	Title      string  `json:"title" bson:"title" valid:"required"`
	Published  bool    `json:"published"`
	TextBody   string  `json:"text-body" bson:"text_body"`
	Comments   HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	Selections HasMany `json:"-" bson:"-" coal:"selections:selections:posts"`
	Note       HasOne  `json:"-" bson:"-" coal:"note:notes:post"`
}

func (p *postModel) Validate() error {
	if p.Title == "error" {
		return errors.New("error")
	}

	return nil
}

type commentModel struct {
	Base    `json:"-" bson:",inline" coal:"comments"`
	Message string         `json:"message"`
	Parent  *bson.ObjectId `json:"-" coal:"parent:comments"`
	Post    bson.ObjectId  `json:"-" bson:"post_id" coal:"post:posts"`
}

type selectionModel struct {
	Base  `json:"-" bson:",inline" coal:"selections:selections"`
	Name  string          `json:"name"`
	Posts []bson.ObjectId `json:"-" bson:"post_ids" coal:"posts:posts"`
}

type noteModel struct {
	Base      `json:"-" bson:",inline" coal:"notes"`
	Title     string    `json:"title" bson:"title" valid:"required"`
	CreatedAt time.Time `json:"created-at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated-at" bson:"updated_at"`
	Post      bson.ObjectId `json:"-" bson:"post_id" coal:"post:posts"`
}

func (n *noteModel) Validate() error {
	ValidateTimestamps(n, "CreatedAt", "UpdatedAt")

	return nil
}
