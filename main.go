package main

import (
	"bytes"
	"fmt"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/url"
)

const (
	accountName = "ストレージアカウント名"
	accountKey = "ストレージキー"
	containerName = "コンテナ名"
)

type blobItem struct {
	Name  string
	Size int64
}

var containerURL azblob.ContainerURL

func main() {
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", accountName, containerName))
	containerURL = azblob.NewContainerURL(*URL, azblob.NewPipeline(credential, azblob.PipelineOptions{}))

	router := gin.Default()
	router.Use(errorMiddleware())
	router.LoadHTMLGlob("templates/*.html")

	router.GET("/", home)
	router.POST("/upload", upload)
	router.POST("/delete", delete)

	router.Run()
}

func home(ctx *gin.Context) {
	var items []blobItem
	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			ctx.Error(err).SetType(gin.ErrorTypePublic)
			return
		}
		marker = listBlob.NextMarker

		for _, blobInfo := range listBlob.Segment.BlobItems {
			items = append(items, blobItem{
				Name: blobInfo.Name,
				Size: *blobInfo.Properties.ContentLength,
			})
		}
	}

	ctx.HTML(200, "index.html", gin.H{
		"items": items,
	})
}

func upload(ctx *gin.Context) {
	form, err := ctx.MultipartForm()
	if err != nil {
		ctx.Error(err).SetType(gin.ErrorTypePublic)
		return
	}

	formFiles := form.File["file1"]
	if formFiles == nil {
		ctx.Redirect(302, "/")
		return
	}
	formFile := formFiles[0]
	fileName := formFile.Filename
	blobURL := containerURL.NewBlockBlobURL(fileName)

	file, err := formFile.Open()
	defer file.Close()
	if err != nil {
		ctx.Error(err).SetType(gin.ErrorTypePublic)
		return
	}
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		ctx.Error(err).SetType(gin.ErrorTypePublic)
		return
	}

	// BLOBにファイルをアップロード
	_, err = azblob.UploadBufferToBlockBlob(ctx, buf.Bytes(), blobURL, azblob.UploadToBlockBlobOptions{
		BlockSize:   4 * 1024 * 1024,
		Parallelism: 16})
	if err != nil {
		ctx.Error(err).SetType(gin.ErrorTypePublic)
		return
	}
	ctx.Redirect(302, "/")
}

func delete(ctx *gin.Context) {
	name := ctx.PostForm("name")
	blobURL := containerURL.NewBlobURL(name)

	// Delete the blob we created earlier.
	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	if err != nil {
		ctx.Error(err).SetType(gin.ErrorTypePublic)
		return
	}
	ctx.Redirect(302, "/")
}

func errorMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		err := ctx.Errors.ByType(gin.ErrorTypePublic).Last()
		if err != nil {
			log.Print(err.Err)
			ctx.HTML(500, "error.html", gin.H{
				"error": err.Error(),
			})
		}
	}
}


