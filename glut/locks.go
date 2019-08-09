package glut

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/256dpi/fire/coal"
)

// Lock will lock the specified value using the specified token for the
// specified duration. Lock may create a new value in the process and lock it
// right away.
func Lock(store *coal.Store, component, name string, token coal.ID, timeout time.Duration) (bool, error) {
	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// get locked
	locked := time.Now().Add(timeout)

	// prepare bulk
	res, err := store.C(&Value{}).BulkWrite(nil, []mongo.WriteModel{
		// init value
		mongo.NewUpdateOneModel().SetFilter(bson.M{
			coal.F(&Value{}, "Component"): component,
			coal.F(&Value{}, "Name"):      name,
		}).SetUpdate(bson.M{
			"$setOnInsert": bson.M{
				coal.F(&Value{}, "Locked"): locked,
				coal.F(&Value{}, "Token"):  token,
			},
		}).SetUpsert(true),

		// lock value
		mongo.NewUpdateOneModel().SetFilter(bson.M{
			"$and": []bson.M{
				{
					coal.F(&Value{}, "Component"): component,
					coal.F(&Value{}, "Name"):      name,
				},
				{
					"$or": []bson.M{
						// unlocked
						{
							coal.F(&Value{}, "Token"): nil,
						},
						// lock timed out
						{
							coal.F(&Value{}, "Locked"): bson.M{
								"$lt": time.Now(),
							},
						},
						// we have the lock
						{
							coal.F(&Value{}, "Token"): token,
						},
					},
				},
			},
		}).SetUpdate(bson.M{
			"$set": bson.M{
				coal.F(&Value{}, "Locked"): locked,
				coal.F(&Value{}, "Token"):  token,
			},
		}),
	})
	if err != nil {
		return false, err
	}

	return res.UpsertedCount > 0 || res.ModifiedCount > 0, nil
}

// SetLocked will update the specified value only if the value is locked by the
// specified token.
func SetLocked(store *coal.Store, component, name string, data []byte, token coal.ID) (bool, error) {
	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// replace value
	res, err := store.C(&Value{}).UpdateOne(nil, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
	}, bson.M{
		"$set": bson.M{
			coal.F(&Value{}, "Data"): data,
		},
	})
	if err != nil {
		return false, err
	}

	return res.ModifiedCount > 0, nil
}

// GetLocked will load the contents of the value with the specified name only
// if the value is locked by the specified token.
func GetLocked(store *coal.Store, component, name string, token coal.ID) ([]byte, bool, error) {
	// find value
	var value *Value
	err := store.C(&Value{}).FindOne(nil, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
	}).Decode(&value)
	if err == mongo.ErrNoDocuments {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	return value.Data, true, nil
}

// DelLocked will update the specified value only if the value is locked by the
// specified token.
func DelLocked(store *coal.Store, component, name string, token coal.ID) (bool, error) {
	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// delete value
	res, err := store.C(&Value{}).DeleteOne(nil, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
	})
	if err != nil {
		return false, err
	}

	return res.DeletedCount > 0, nil
}

// Unlock will unlock the specified value if the provided token does match.
func Unlock(store *coal.Store, component, name string, token coal.ID) (bool, error) {
	// check token
	if token.IsZero() {
		return false, fmt.Errorf("invalid token")
	}

	// replace value
	res, err := store.C(&Value{}).UpdateOne(nil, bson.M{
		coal.F(&Value{}, "Component"): component,
		coal.F(&Value{}, "Name"):      name,
		coal.F(&Value{}, "Token"):     token,
	}, bson.M{
		"$set": bson.M{
			coal.F(&Value{}, "Locked"): nil,
			coal.F(&Value{}, "Token"):  nil,
		},
	})
	if err != nil {
		return false, err
	}

	return res.ModifiedCount > 0, nil
}
