package fire

import (
	"errors"
	"reflect"

	"github.com/asaskevich/govalidator"
	"gopkg.in/mgo.v2/bson"
)

// A Callback can be an Authorizer or Validator and is called during execution of
// a controller.
//
// Note: If the callback returns an error wrapped using Fatal() the API returns
// an InternalServerError status and the error will be logged. All other errors
// are serialized to an error object and returned.
type Callback func(*Context) error

type fatalError struct {
	err error
}

// Fatal wraps an error and marks it as fatal.
func Fatal(err error) error {
	return &fatalError{
		err: err,
	}
}

func (err *fatalError) Error() string {
	return err.err.Error()
}

func isFatal(err error) bool {
	_, ok := err.(*fatalError)
	return ok
}

// Combine combines multiple callbacks to one.
//
// Note: Execution will be stopped if a callback returns and error.
func Combine(callbacks ...Callback) Callback {
	return func(ctx *Context) error {
		// call all callbacks
		for _, cb := range callbacks {
			err := cb(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// ModelValidator uses the govalidator package to validate the model based on
// the "valid" struct tags.
func ModelValidator() Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// validate id
		if !ctx.Model.ID().Valid() {
			return errors.New("Invalid ID")
		}

		// TODO: Add error source pointer.

		// validate model
		_, err := govalidator.ValidateStruct(ctx.Model)
		if err != nil {
			return err
		}

		return nil
	}
}

// ProtectedAttributesValidator compares protected attributes against their
// default (during Create) or stored value (during Update) and returns and
// error if they have been changed.
//
// Attributes are defined by passing pairs of fields and default values:
//
//		ProtectedAttributesValidator(fire.Map{
//			"title": "A fixed title",
//		})
//
func ProtectedAttributesValidator(attributes map[string]interface{}) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		if ctx.Action == Create {
			// check all attributes
			for field, def := range attributes {
				if !reflect.DeepEqual(ctx.Model.MustGet(field), def) {
					return errors.New("Field " + field + " is protected")
				}
			}
		}

		if ctx.Action == Update {
			// read the original
			original, err := ctx.Original()
			if err != nil {
				return Fatal(err)
			}

			// check all attributes
			for field := range attributes {
				if !reflect.DeepEqual(ctx.Model.MustGet(field), original.MustGet(field)) {
					return errors.New("Field " + field + " is protected")
				}
			}
		}

		return nil
	}
}

// DependentResourcesValidator counts documents in the supplied collections
// and returns an error if some get found. This callback is meant to protect
// resources from breaking relations when requested to be deleted.
//
// Resources are defined by passing pairs of collections and fields where the
// field must be a database field of the target resource model:
//
//		DependentResourcesValidator(fire.Map{
// 			"posts": "user_id",
//			"comments": "user_id",
// 		})
//
func DependentResourcesValidator(resources map[string]string) Callback {
	return func(ctx *Context) error {
		// only run validator on Delete
		if ctx.Action != Delete {
			return nil
		}

		// check all relations
		for coll, field := range resources {
			// count referencing documents
			n, err := ctx.Store.DB().C(coll).Find(bson.M{
				field: ctx.Query["_id"],
			}).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}

			// immediately return if a document is found
			if n == 1 {
				return errors.New("Resource has dependent resources")
			}
		}

		// pass validation
		return nil
	}
}

// VerifyReferencesValidator makes sure all references in the document are
// existing by counting on the related collections.
//
// References are defined by passing pairs of fields and collections where the
// field must be a database field on the resource model:
//
//		VerifyReferencesValidator(fire.Map{
// 			"post_id": "posts",
//			"user_id": "users",
// 		})
//
func VerifyReferencesValidator(references map[string]string) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// check all references
		for field, collection := range references {
			// read referenced id
			id := ctx.Model.MustGet(field)

			// continue if reference is not set
			if oid, ok := id.(*bson.ObjectId); ok && oid == nil {
				continue
			}

			// count entities in database
			n, err := ctx.Store.DB().C(collection).FindId(id).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}

			// check for existence
			if n != 1 {
				return errors.New("Missing required relationship " + field)
			}
		}

		// pass validation
		return nil
	}
}

// MatchingReferencesValidator compares the model with a related model and
// checks if certain references are shared.
//
// The target model is defined by passing its collection and the referencing
// field on the current model. The matcher is defined by passing pairs of
// database fields on the target and current model:
//
//		MatchingReferencesValidator("posts", "post_id", fire.Map{
// 			"user_id": "user_id",
// 		})
//
func MatchingReferencesValidator(collection, reference string, matcher map[string]string) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// get main reference
		id := ctx.Model.MustGet(reference)

		// continue if reference is not set
		if oid, ok := id.(*bson.ObjectId); ok && oid == nil {
			return nil
		}

		// prepare query
		query := bson.M{
			"_id": id,
		}

		// add other references
		for targetField, modelField := range matcher {
			id := ctx.Model.MustGet(modelField)

			// abort if reference is missing
			if oid, ok := id.(*bson.ObjectId); ok && oid == nil {
				return errors.New("Missing ID")
			}

			query[targetField] = id
		}

		// query db
		n, err := ctx.Store.DB().C(collection).Find(query).Limit(1).Count()
		if err != nil {
			return Fatal(err)
		}

		// return error if document is missing
		if n == 0 {
			return errors.New("References do not match")
		}

		return nil
	}
}