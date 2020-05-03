package blaze

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/256dpi/serve"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

// Storage provides file storage services.
type Storage struct {
	store   *coal.Store
	notary  *heat.Notary
	service Service
}

// NewStorage creates a new storage.
func NewStorage(store *coal.Store, notary *heat.Notary, service Service) *Storage {
	return &Storage{
		store:   store,
		notary:  notary,
		service: service,
	}
}

// Upload will uploaded the provided stream using the configured service and
// return a claim key.
func (s *Storage) Upload(ctx context.Context, contentType string, cb func(Upload) (int64, error)) (string, *File, error) {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.Upload")
	span.Log("contentType", contentType)
	defer span.Finish()

	// set default content type
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// create handle
	handle, err := s.service.Prepare(ctx)
	if err != nil {
		return "", nil, err
	}

	// prepare file
	file := &File{
		Base:    coal.B(),
		State:   Uploading,
		Updated: time.Now(),
		Type:    contentType,
		Handle:  handle,
	}

	// create file
	err = s.store.M(file).Insert(ctx, file)
	if err != nil {
		return "", nil, err
	}

	// begin upload
	upload, err := s.service.Upload(ctx, handle, contentType)
	if err != nil {
		return "", nil, err
	}

	// perform upload
	length, err := cb(upload)
	if err != nil {
		return "", nil, err
	}

	// get time
	now := time.Now()

	// update file
	_, err = s.store.M(file).Update(ctx, nil, file.ID(), bson.M{
		"$set": bson.M{
			"State":   Uploaded,
			"Updated": now,
			"Length":  length,
		},
	}, false)
	if err != nil {
		return "", nil, err
	}

	// set fields
	file.State = Uploaded
	file.Updated = now
	file.Length = length

	// issue claim key
	claimKey, err := s.notary.Issue(&ClaimKey{
		File: file.ID(),
	})
	if err != nil {
		return "", nil, err
	}

	return claimKey, file, nil
}

// UploadAction returns an action that provides and upload that service that
// stores blobs and returns claim keys.
func (s *Storage) UploadAction(limit int64) *fire.Action {
	// set default limit
	if limit == 0 {
		limit = serve.MustByteSize("8M")
	}

	return fire.A("blaze/Storage.UploadAction", []string{"POST"}, limit, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
		}

		// get content type
		contentType, ctParams, err := mime.ParseMediaType(ctx.HTTPRequest.Header.Get("Content-Type"))
		if err != nil {
			ctx.ResponseWriter.WriteHeader(http.StatusBadRequest)
			return nil
		}

		// get content length
		contentLength := ctx.HTTPRequest.ContentLength
		if contentLength != -1 && contentLength > limit {
			ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return nil
		}

		// upload multipart or raw
		var keys []string
		if contentType == "multipart/form-data" {
			keys, err = s.uploadMultipart(ctx, ctParams["boundary"])
		} else {
			keys, err = s.uploadBody(ctx, contentType)
		}

		// check limit error
		if err != nil && strings.HasSuffix(err.Error(), serve.ErrBodyLimitExceeded.Error()) {
			ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return nil
		} else if err != nil {
			return err
		}

		// respond with keys
		return ctx.Respond(stick.Map{
			"keys": keys,
		})
	})
}

func (s *Storage) uploadBody(ctx *fire.Context, contentType string) ([]string, error) {
	// trace
	ctx.Trace.Push("blaze/Storage.uploadBody")
	defer ctx.Trace.Pop()

	// upload stream
	claimKey, _, err := s.Upload(ctx, contentType, func(upload Upload) (int64, error) {
		return UploadFrom(upload, ctx.HTTPRequest.Body)
	})
	if err != nil {
		return nil, err
	}

	return []string{claimKey}, nil
}

func (s *Storage) uploadMultipart(ctx *fire.Context, boundary string) ([]string, error) {
	// trace
	ctx.Trace.Push("blaze/Storage.uploadMultipart")
	defer ctx.Trace.Pop()

	// prepare reader
	reader := multipart.NewReader(ctx.HTTPRequest.Body, boundary)

	// get first part
	part, err := reader.NextPart()
	if err != nil && err != io.EOF {
		return nil, err
	}

	// collect claim keys
	var claimKeys []string

	// handle all parts
	for part != nil {
		// parse content type
		contentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}

		// upload part
		claimKey, _, err := s.Upload(ctx, contentType, func(upload Upload) (int64, error) {
			return UploadFrom(upload, part)
		})
		if err != nil {
			return nil, err
		}

		// add claim key
		claimKeys = append(claimKeys, claimKey)

		// get next part
		part, err = reader.NextPart()
		if err != nil && err != io.EOF {
			return nil, err
		}
	}

	return claimKeys, nil
}

// Validator will validate all or just the specified link fields of the model.
func (s *Storage) Validator(fields ...string) *fire.Callback {
	return fire.C("blaze/Storage.Validator", fire.Only(fire.Create, fire.Update, fire.Delete), func(ctx *fire.Context) error {
		// check store
		if ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
		}

		// collect fields if empty
		if len(fields) == 0 {
			fields = collectFields(ctx.Controller.Model)
		}

		// check all fields
		for _, field := range fields {
			// get value
			value := stick.MustGet(ctx.Model, field)

			// get old value
			var oldValue interface{}
			if ctx.Original != nil {
				oldValue = stick.MustGet(ctx.Original, field)
			}

			// get path
			path := coal.GetMeta(ctx.Model).Fields[field].JSONKey

			// inspect type
			var err error
			switch value := value.(type) {
			case Link:
				var oldLink *Link
				if oldValue != nil {
					r := oldValue.(Link)
					oldLink = &r
				}
				newLink := &value
				if ctx.Operation == fire.Delete {
					oldLink = newLink
					newLink = nil
				}
				err = s.validateLink(ctx, newLink, oldLink, path)
				stick.MustSet(ctx.Model, field, value)
			case *Link:
				var oldLink *Link
				if oldValue != nil {
					oldLink = oldValue.(*Link)
				}
				newLink := value
				if ctx.Operation == fire.Delete {
					oldLink = newLink
					newLink = nil
				}
				err = s.validateLink(ctx, newLink, oldLink, path)
				stick.MustSet(ctx.Model, field, newLink)
			default:
				err = fmt.Errorf("unsupported type")
			}
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Storage) validateLink(ctx context.Context, newLink, oldLink *Link, path string) error {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.validateLink")
	defer span.Finish()

	// detect change
	added := oldLink == nil && newLink != nil
	updated := oldLink != nil && newLink != nil && newLink.ClaimKey != ""
	deleted := oldLink != nil && newLink == nil

	// check if changed
	if !added && !updated && !deleted {
		return nil
	}

	// claim new file
	if added || updated {
		// check claim
		if newLink.ClaimKey == "" {
			return fmt.Errorf("%s: missing claim key", path)
		}

		// verify claim key
		var claimKey ClaimKey
		err := s.notary.Verify(&claimKey, newLink.ClaimKey)
		if err != nil {
			return err
		}

		// claim new file
		var newFile File
		found, err := s.store.M(&File{}).UpdateFirst(ctx, &newFile, bson.M{
			"_id":   claimKey.File,
			"State": Uploaded,
		}, bson.M{
			"$set": bson.M{
				"State":   Claimed,
				"Updated": time.Now(),
			},
		}, nil, false)
		if err != nil {
			return err
		} else if !found {
			return fmt.Errorf("%s: unable to claim file", path)
		}

		// update link
		newLink.Type = newFile.Type
		newLink.Length = newFile.Length
		newLink.File = coal.P(newFile.ID())
	}

	// release old file
	if updated || deleted {
		found, err := s.store.M(&File{}).UpdateFirst(ctx, nil, bson.M{
			"_id":   oldLink.File,
			"State": Claimed,
		}, bson.M{
			"$set": bson.M{
				"State":   Released,
				"Updated": time.Now(),
			},
		}, nil, false)
		if err != nil {
			return err
		} else if !found {
			return fmt.Errorf("%s: unable to release file", path)
		}
	}

	return nil
}

// Decorator will generate view keys for all or just the specified link fields
// on the returned model or models.
func (s *Storage) Decorator(fields ...string) *fire.Callback {
	return fire.C("blaze/Storage.Decorator", fire.All(), func(ctx *fire.Context) error {
		// collect fields if empty
		if len(fields) == 0 {
			fields = collectFields(ctx.Controller.Model)
		}

		// decorate model
		if ctx.Model != nil {
			err := s.decorateModel(ctx.Model, fields)
			if err != nil {
				return err
			}
		}

		// decorate models
		for _, model := range ctx.Models {
			err := s.decorateModel(model, fields)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Storage) decorateModel(model coal.Model, fields []string) error {
	// collect fields if empty
	if len(fields) == 0 {
		fields = collectFields(model)
	}

	// iterate over all fields
	for _, field := range fields {
		// get value
		value := stick.MustGet(model, field)

		// inspect type
		var err error
		switch value := value.(type) {
		case Link:
			err = s.decorateLink(&value)
			stick.MustSet(model, field, value)
		case *Link:
			err = s.decorateLink(value)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) decorateLink(link *Link) error {
	// skip if file is missing
	if link == nil || link.File == nil || link.File.IsZero() {
		return nil
	}

	// issue view key
	viewKey, err := s.notary.Issue(&ViewKey{
		File: *link.File,
	})
	if err != nil {
		return err
	}

	// set key
	link.ViewKey = viewKey

	return nil
}

// Download will initiate a download for the blob referenced by the provided
// view key.
func (s *Storage) Download(ctx context.Context, viewKey string) (Download, *File, error) {
	// verify key
	var key ViewKey
	err := s.notary.Verify(&key, viewKey)
	if err != nil {
		return nil, nil, err
	}

	// load file
	var file File
	found, err := s.store.M(&File{}).FindFirst(ctx, &file, bson.M{
		"_id":   key.File,
		"State": Claimed,
	}, nil, 0, false)
	if err != nil {
		return nil, nil, err
	} else if !found {
		return nil, nil, fmt.Errorf("missing file")
	}

	// begin download
	download, err := s.service.Download(ctx, file.Handle)
	if err != nil {
		return nil, nil, err
	}

	return download, &file, nil
}

// DownloadAction returns an action that allows downloading files using view
// keys.
func (s *Storage) DownloadAction() *fire.Action {
	return fire.A("blaze/Storage.DownloadAction", []string{"GET"}, 0, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
		}

		// initiate download
		download, file, err := s.Download(ctx, ctx.HTTPRequest.URL.Query().Get("key"))
		if err != nil {
			return err
		}

		// set content type and length
		ctx.ResponseWriter.Header().Set("Content-Type", file.Type)
		ctx.ResponseWriter.Header().Set("Content-Length", strconv.FormatInt(file.Length, 10))

		// download file
		err = DownloadTo(download, ctx.ResponseWriter)
		if err != nil {
			return err
		}

		return nil
	})
}

// CleanupTask will return a periodic task that can be run to periodically
// cleanup obsolete files.
func (s *Storage) CleanupTask(periodicity, retention time.Duration) *axe.Task {
	return &axe.Task{
		Job: &CleanupJob{},
		Handler: func(ctx *axe.Context) error {
			return s.Cleanup(ctx, retention)
		},
		Workers:     1,
		MaxAttempts: 1,
		Lifetime:    periodicity,
		Periodicity: periodicity,
		PeriodicJob: axe.Blueprint{
			Job: &CleanupJob{
				Base: axe.B("periodic"),
			},
		},
	}
}

// Cleanup will remove obsolete files and remove their blobs. Files in the
// states "uploading" or "uploaded" are removed after the specified retention
// which defaults to 1 hour if zero. Files in the states "released" and
// "deleting" are removed immediately. It will also allow the service to cleanup.
func (s *Storage) Cleanup(ctx context.Context, retention time.Duration) error {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.Cleanup")
	span.Log("retention", retention.String())
	defer span.Finish()

	// set default retention
	if retention == 0 {
		retention = time.Hour
	}

	// get iterator for deletable files
	iter, err := s.store.M(&File{}).FindEach(ctx, bson.M{
		"$or": []bson.M{
			{
				"State": bson.M{
					"$in": bson.A{Uploading, Uploaded},
				},
				"Updated": bson.M{
					"$lt": time.Now().Add(-retention),
				},
			},
			{
				"State": bson.M{
					"$in": bson.A{Released, Deleting},
				},
			},
		},
	}, nil, 0, 0, false, coal.Unsafe)
	if err != nil {
		return err
	}

	// iterate over files
	defer iter.Close()
	for iter.Next() {
		// decode file
		var file File
		err := iter.Decode(&file)
		if err != nil {
			return err
		}

		// flag file as deleting if not already
		if file.State != Deleting {
			found, err := s.store.M(&File{}).UpdateFirst(ctx, nil, bson.M{
				"_id":   file.ID(),
				"State": file.State,
			}, bson.M{
				"$set": bson.M{
					"State":   Deleting,
					"Updated": time.Now(),
				},
			}, nil, false)
			if err != nil {
				return err
			} else if !found {
				return nil
			}
		}

		// delete blob
		deleted, err := s.service.Delete(ctx, file.Handle)
		if err != nil {
			return err
		}

		// delete file if blob has been deleted
		if deleted {
			_, err = s.store.M(&File{}).Delete(ctx, nil, file.ID())
			if err != nil {
				return err
			}
		}
	}

	// check error
	err = iter.Error()
	if err != nil {
		return err
	}

	// cleanup service
	err = s.service.Cleanup(ctx)
	if err != nil {
		return err
	}

	return nil
}
