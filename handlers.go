package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/context"
	"github.com/julienschmidt/httprouter"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//
// BucketMeta represents a named group of objects
//
type BucketMeta struct {
	Name    string       `json:"name"`
	Objects []ObjectMeta `json:"objects"`
}

//
// ObjectMeta represents stored object meta information
//
type ObjectMeta struct {
	ID          bson.ObjectId          `bson:"_id" json:"id"`
	Filename    string                 `bson:"filename" json:"filename"`
	ContentType string                 `bson:"contentType" json:"content_type"`
	Size        int64                  `bson:"length" json:"size"`
	ChunkSize   int64                  `bson:"chunkSize" json:"chunk_size"`
	CheckSum    string                 `bson:"md5" json:"md5"`
	CreatedOn   time.Time              `bson:"uploadDate" json:"created_on"`
	Metadata    map[string]interface{} `bson:"metadata,omitempty" json:"extra,omitempty"`
}

//
// BucketWebHandler is a collection of CRUD methods of Bucket API
//
type BucketWebHandler struct {
	Session *mgo.Session
	log     *log.Logger
}

// NewBucketWebHandler creates new BucketWebHandler
func NewBucketWebHandler() *BucketWebHandler {
	return &BucketWebHandler{
		log: log.New(os.Stdout, "[BucketHandler] ", log.LstdFlags),
	}
}

// CheckExists handles the head request to check if bucket exists
func (handler *BucketWebHandler) CheckExists(w http.ResponseWriter, r *http.Request) {
	params := context.Get(r, CtxParamsKey).(httprouter.Params)
	if params.ByName("name") == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid bucket name", nil)
		return
	}

	// Ensure we have index on bucket name
	err := handler.Session.DB(os.Getenv(EnvGridFSDatabase)).C(fmt.Sprintf("%v.files", os.Getenv(EnvGridFSPrefix))).EnsureIndexKey("metadata.bucket")
	if err != nil {
		handler.log.Printf("WARNING: Failed to ensure index: %v", err.Error())
	}

	fs := handler.Session.DB(os.Getenv(EnvGridFSDatabase)).GridFS(os.Getenv(EnvGridFSPrefix))

	n, err := fs.Find(bson.M{"metadata.bucket": params.ByName("name")}).Count()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Operational error", err)
		return
	}

	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusFound)
}

// Retreive handles the get bucket info request
func (handler *BucketWebHandler) Retreive(w http.ResponseWriter, r *http.Request) {
	params := context.Get(r, CtxParamsKey).(httprouter.Params)
	if params.ByName("name") == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid bucket name", nil)
		return
	}

	fs := handler.Session.DB(os.Getenv(EnvGridFSDatabase)).GridFS(os.Getenv(EnvGridFSPrefix))

	var meta ObjectMeta
	bucket := &BucketMeta{}
	bucket.Name = params.ByName("name")
	bucket.Objects = []ObjectMeta{}

	iter := fs.Find(bson.M{"metadata.bucket": params.ByName("name")}).Iter()
	for iter.Next(&meta) {
		// drop bucket name
		if _, ok := meta.Metadata["bucket"]; ok {
			delete(meta.Metadata, "bucket")
		}
		bucket.Objects = append(bucket.Objects, meta)
	}
	if err := iter.Close(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Operational error", err)
		return
	}

	json.NewEncoder(w).Encode(bucket)
}

// Delete handles the delete bucket and all included object request
func (handler *BucketWebHandler) Delete(w http.ResponseWriter, r *http.Request) {
	respondWithError(w, http.StatusNotImplemented, "Not implemented", nil)
}

//
// ObjectWebHandler is a collection of CRUD methods for Objects API
//
type ObjectWebHandler struct {
	Session *mgo.Session
	log     *log.Logger
}

// NewObjectWebHandler creates new SessionWebHandler
func NewObjectWebHandler() *ObjectWebHandler {
	return &ObjectWebHandler{
		log: log.New(os.Stdout, "[ObjectHandler] ", log.LstdFlags),
	}
}

// Create handles multipart upload of meta data and blob
func (handler *ObjectWebHandler) Create(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		part  *multipart.Part
		value []byte
		file  *mgo.GridFile
	)

	// We respond as JSON
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Check if the content type is correct
	if ctype := r.Header.Get("Content-Type"); !strings.HasPrefix(ctype, "multipart/form-data") {
		respondWithError(w, http.StatusUnsupportedMediaType, "Unsupported media type", fmt.Errorf("Expecting multipart/form-data content type but received: %v", ctype))
		return
	}

	body, err := r.MultipartReader()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to parse data", err)
		return
	}

	fs := handler.Session.DB(os.Getenv(EnvGridFSDatabase)).GridFS(os.Getenv(EnvGridFSPrefix))

	obj := &ObjectMeta{}
	obj.Metadata = map[string]interface{}{}
	obj.Metadata["cid"] = r.Header.Get("X-Correlation-Id")

	for part, err = body.NextPart(); err == nil; part, err = body.NextPart() {
		if part.FormName() == "filename" && part.FileName() == "" {
			value, err = ioutil.ReadAll(part)
			if err != nil {
				break
			}
			obj.Filename = string(value)
		} else if part.FormName() == "content_type" && part.FileName() == "" {
			value, err = ioutil.ReadAll(part)
			if err != nil {
				break
			}
			obj.ContentType = string(value)
		} else if part.FormName() == "extra.bucket" && part.FileName() == "" {
			value, err = ioutil.ReadAll(part)
			if err != nil {
				break
			}
			obj.Metadata["bucket"] = string(value)
		} else if part.FormName() == "object" && part.FileName() != "" {
			file, err = fs.Create(part.FileName())
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to create GridFS file", err)
				return
			}
			_, err = io.Copy(file, part)
			if err != nil {
				file.Close()
				respondWithError(w, http.StatusInternalServerError, "Failed to save GridFS file", err)
				return
			}
		}
	}
	if err != nil && err != io.EOF {
		if file != nil {
			file.Close()
		}
		respondWithError(w, http.StatusBadRequest, "Failed to process multipart form", err)
		return
	} else if file == nil {
		respondWithError(w, http.StatusBadRequest, "Bad request", fmt.Errorf("No object has been uploaded"))
		return
	}

	// Update metadata
	file.SetName(obj.Filename)
	file.SetMeta(obj.Metadata)
	if obj.ContentType != "" {
		file.SetContentType(obj.ContentType)
	}
	err = file.Close()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to close GridFS file", err)
		return
	}
	obj.ID = file.Id().(bson.ObjectId)

	// Read newly created meta & return it
	err = fs.Find(bson.M{"_id": obj.ID}).One(&obj)
	if err == mgo.ErrNotFound {
		respondWithError(w, http.StatusInternalServerError, "Newly created could not be found", err)
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Operational error", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(obj)
}

// Update handles the update of the meta data of a given object
func (handler *ObjectWebHandler) Update(w http.ResponseWriter, r *http.Request) {
	respondWithError(w, http.StatusNotImplemented, "Not implemented", nil)
}

// CheckExists handles the head request to check if object exists
func (handler *ObjectWebHandler) CheckExists(w http.ResponseWriter, r *http.Request) {
	params := context.Get(r, CtxParamsKey).(httprouter.Params)
	if !bson.IsObjectIdHex(params.ByName("id")) {
		respondWithError(w, http.StatusBadRequest, "Invalid object ID", nil)
		return
	}

	objID := bson.ObjectIdHex(params.ByName("id"))
	fs := handler.Session.DB(os.Getenv(EnvGridFSDatabase)).GridFS(os.Getenv(EnvGridFSPrefix))

	var result interface{}
	err := fs.Find(bson.M{"_id": objID}).One(&result)
	if err == mgo.ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Operational error", err)
		return
	}

	w.WriteHeader(http.StatusFound)
}

// Download handles the files download
func (handler *ObjectWebHandler) Download(w http.ResponseWriter, r *http.Request) {
	params := context.Get(r, CtxParamsKey).(httprouter.Params)
	if !bson.IsObjectIdHex(params.ByName("id")) {
		respondWithError(w, http.StatusBadRequest, "Invalid object ID", nil)
		return
	}

	objID := bson.ObjectIdHex(params.ByName("id"))
	fs := handler.Session.DB(os.Getenv(EnvGridFSDatabase)).GridFS(os.Getenv(EnvGridFSPrefix))

	file, err := fs.OpenId(objID)
	if err == mgo.ErrNotFound {
		respondWithError(w, http.StatusNotFound, "Object does not exist", err)
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Operational error", err)
		return
	}
	defer file.Close()

	if file.ContentType() != "" {
		w.Header().Set("Content-Type", file.ContentType())
	}
	if file.Name() != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", file.Name()))
	}

	_, err = io.Copy(w, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Operational error", err)
		return
	}
}

// About handles requests to read meta data
func (handler *ObjectWebHandler) About(w http.ResponseWriter, r *http.Request) {
	params := context.Get(r, CtxParamsKey).(httprouter.Params)
	if !bson.IsObjectIdHex(params.ByName("id")) {
		respondWithError(w, http.StatusBadRequest, "Invalid object ID", nil)
		return
	}

	objID := bson.ObjectIdHex(params.ByName("id"))
	fs := handler.Session.DB(os.Getenv(EnvGridFSDatabase)).GridFS(os.Getenv(EnvGridFSPrefix))

	var meta ObjectMeta
	err := fs.Find(bson.M{"_id": objID}).One(&meta)
	if err == mgo.ErrNotFound {
		respondWithError(w, http.StatusNotFound, "Object does not exist", err)
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Operational error", err)
		return
	}

	json.NewEncoder(w).Encode(meta)
}

// Delete handles objects deletion
func (handler *ObjectWebHandler) Delete(w http.ResponseWriter, r *http.Request) {
	respondWithError(w, http.StatusNotImplemented, "Not implemented", nil)
}
