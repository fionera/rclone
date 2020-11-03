package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/madmin"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

var (
	errNotSupported = fmt.Errorf("not supported")
)

type fsLayer struct {
	minio.GatewayUnsupported

	creds auth.Credentials
	vfs   *vfs.VFS
}

func (f *fsLayer) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result minio.ListMultipartsInfo, err error) {
	panic("implement me")
}

func (f *fsLayer) NewMultipartUpload(ctx context.Context, bucket, object string, opts minio.ObjectOptions) (uploadID string, err error) {
	panic("implement me")
}

func (f *fsLayer) PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data *minio.PutObjReader, opts minio.ObjectOptions) (info minio.PartInfo, err error) {
	panic("implement me")
}

func (f *fsLayer) GetMultipartInfo(ctx context.Context, bucket, object, uploadID string, opts minio.ObjectOptions) (info minio.MultipartInfo, err error) {
	panic("implement me")
}

func (f *fsLayer) ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker int, maxParts int, opts minio.ObjectOptions) (result minio.ListPartsInfo, err error) {
	panic("implement me")
}

func (f *fsLayer) AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string, opts minio.ObjectOptions) error {
	panic("implement me")
}

func (f *fsLayer) CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, uploadedParts []minio.CompletePart, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	panic("implement me")
}

func NewFSLayer(creds auth.Credentials, vfs *vfs.VFS) (minio.ObjectLayer, error) {
	return &fsLayer{
		creds: creds,
		vfs:   vfs,
	}, nil
}

func (f *fsLayer) Shutdown(ctx context.Context) error {
	return nil
}

func (f *fsLayer) StorageInfo(ctx context.Context, local bool) (minio.StorageInfo, []error) {
	return minio.StorageInfo{
		Backend: struct {
			Type             minio.BackendType
			GatewayOnline    bool
			OnlineDisks      madmin.BackendDisks
			OfflineDisks     madmin.BackendDisks
			StandardSCData   int
			StandardSCParity int
			RRSCData         int
			RRSCParity       int
		}{Type: minio.BackendGateway, GatewayOnline: true},
	}, nil
}

func (f *fsLayer) MakeBucketWithLocation(ctx context.Context, bucket string, opts minio.BucketOptions) error {
	if bucket == f.vfs.Fs().Name() {
		return nil
	}
	return errNotSupported
}

func (f *fsLayer) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo minio.BucketInfo, err error) {
	return minio.BucketInfo{
		Name:    bucket,
		Created: time.Unix(0, 0),
	}, nil
}

func (f *fsLayer) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	return []minio.BucketInfo{
		{
			Name:    f.vfs.Fs().Name(),
			Created: time.Unix(0, 0),
		},
	}, nil
}

func (f *fsLayer) DeleteBucket(ctx context.Context, bucket string, forceDelete bool) error {
	return errNotSupported
}

func (f *fsLayer) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	dir, err := f.vfs.ReadDir(prefix)
	if err != nil {
		return result, err
	}

	var objInfos []minio.ObjectInfo
	for _, info := range dir {
		path := filepath.Join(prefix, info.Name())

		objInfo := minio.ObjectInfo{
			Bucket:  bucket,
			Name:    path,
			ModTime: info.ModTime(),
		}

		if info.IsDir() {
			objInfo.IsDir = true
		} else {
			objInfo.Size = info.Size()
		}

		objInfos = append(objInfos, objInfo)
	}

	for _, objInfo := range objInfos {
		if objInfo.IsDir && delimiter == minio.SlashSeparator {
			result.Prefixes = append(result.Prefixes, objInfo.Name+minio.SlashSeparator)
			continue
		}

		result.Objects = append(result.Objects, objInfo)
	}

	return result, nil
}

func (f *fsLayer) GetObjectNInfo(ctx context.Context, bucket, object string, rs *minio.HTTPRangeSpec, h http.Header, lockType minio.LockType, opts minio.ObjectOptions) (*minio.GetObjectReader, error) {
	node, err := f.vfs.Stat(object)
	if err != nil {
		return nil, err
	}

	objectInfo := minio.ObjectInfo{
		Bucket:  bucket,
		Name:    node.Name(),
		ModTime: node.ModTime(),
		Size:    node.Size(),
		IsDir:   node.IsDir(),
	}

	if node.IsDir() {
		return minio.NewGetObjectReaderFromReader(bytes.NewBuffer(nil), objectInfo, opts)
	}

	hdl, err := f.vfs.Open(object)
	if err != nil {
		return nil, err
	}

	return minio.NewGetObjectReaderFromReader(hdl, objectInfo, opts, func() {
		hdl.Close()
	})
}

func (f *fsLayer) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts minio.ObjectOptions) (err error) {
	node, err := f.vfs.Stat(object)
	if err != nil {
		return err
	}

	if node.IsDir() {
		_, err = writer.Write([]byte(""))
		return err
	}

	hdl, err := f.vfs.Open(object)
	if err != nil {
		return err
	}
	defer hdl.Close()

	_, err = io.Copy(writer, hdl)
	if err != nil {
		return err
	}

	return nil
}

func (f *fsLayer) GetObjectInfo(ctx context.Context, bucket, object string, opts minio.ObjectOptions) (minio.ObjectInfo, error) {
	node, err := f.vfs.Stat(object)
	if err != nil {
		if err == vfs.ENOENT {
			err = minio.ObjectNotFound{
				Bucket:    bucket,
				Object:    object,
			}
		}
		fs.Errorf(nil, "not found: %v", err)
		return minio.ObjectInfo{}, err
	}

	return minio.ObjectInfo{
		Bucket:  bucket,
		Name:    node.Name(),
		ModTime: node.ModTime(),
		Size:    node.Size(),
		IsDir:   node.IsDir(),
	}, nil
}

func (f *fsLayer) PutObject(ctx context.Context, bucket, object string, data *minio.PutObjReader, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	hdl, err := f.vfs.Create(object)
	if err != nil {
		return objInfo, err
	}

	if _, err := io.Copy(hdl, data); err != nil {
		return objInfo, err
	}

	if err := hdl.Flush(); err != nil {
		return objInfo, err
	}

	return minio.ObjectInfo{
		Name:   hdl.Name(),
		Bucket: bucket,
	}, nil
}

func (f *fsLayer) DeleteObject(ctx context.Context, bucket, object string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	if err := f.vfs.Remove(object); err != nil {
		return objInfo, err
	}

	return minio.ObjectInfo{Bucket: bucket, Name: object}, nil
}

func (f *fsLayer) DeleteObjects(ctx context.Context, bucket string, objects []minio.ObjectToDelete, opts minio.ObjectOptions) ([]minio.DeletedObject, []error) {
	errs := make([]error, len(objects))
	dobjects := make([]minio.DeletedObject, len(objects))

	for idx, object := range objects {
		_, err := f.DeleteObject(ctx, bucket, object.ObjectName, opts)
		if err != nil {
			errs[idx] = err
			continue
		}

		dobjects[idx] = minio.DeletedObject{ObjectName: object.ObjectName}
	}

	return dobjects, errs
}

func (f *fsLayer) Walk(ctx context.Context, bucket, prefix string, results chan<- minio.ObjectInfo, opts minio.ObjectOptions) error {
	go func() {
		if err := f.doWalk(ctx, bucket, prefix, results, opts); err != nil {
			fs.Errorf(f.vfs.Fs(), "error while tree walking: %v", err)
		}
		close(results)
	}()

	return nil
}

func (f *fsLayer) doWalk(ctx context.Context, bucket, prefix string, results chan<- minio.ObjectInfo, opts minio.ObjectOptions) error {
	dir, err := f.vfs.ReadDir(prefix)
	if err != nil {
		return err
	}

	for _, info := range dir {
		path := filepath.Join(prefix, info.Name())

		if info.IsDir() {
			if err := f.doWalk(ctx, bucket, path, results, opts); err != nil {
				fmt.Println(path)
				os.Exit(1)
				return err
			}
		} else {
			results <- minio.ObjectInfo{
				Bucket:  bucket,
				Name:    path,
				ModTime: info.ModTime(),
				Size:    info.Size(),
			}
		}
	}

	return nil
}
