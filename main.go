package main

import (
	"flag"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"log"
	"net/http"
	"net/url"
	"os"
)

var tarURI = flag.String("tar-uri", "", `URI to tar file (ex "file://file.tar" or "https://example.com/file.tar")`)
var outputImageTagName = flag.String("output-tag", "", "tag for output image")
var baseImageTagName = flag.String("base-tag", "", "tag for base image (optional)")
var useDaemon = flag.Bool("daemon", true, "Publish image to local daemon instead of remote registry (optional)")
var useRemote = flag.Bool("remote", false, "Publish image to remote registry (optional)")

func main() {
	flag.Parse()

	if *tarURI == "" || *outputImageTagName == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(*tarURI, *baseImageTagName, *outputImageTagName, *useDaemon, *useRemote); err != nil {
		log.Fatal(err)
	}
}

func run(tarURL, baseImageName, outputImageName string, useDaemon, useRemote bool) error {
	outputImageRef, err := name.ParseReference(outputImageName, name.WeakValidation)
	if err != nil {
		return err
	}

	image := empty.Image
	if baseImageName != "" {
		baseImageRef, err := name.ParseReference(baseImageName, name.WeakValidation)
		if err != nil {
			return err
		}

		if useDaemon {
			baseImageTag, err := name.NewTag(baseImageRef.Name())
			if err != nil {
				return err
			}

			image, err = daemon.Image(baseImageTag)
			if err != nil {
				return err
			}
		}

		if useRemote {
			image, err = remote.Image(baseImageRef)
			if err != nil {
				return err
			}
		}
	}

	parsedURL, err := url.ParseRequestURI(tarURL)
	if err != nil {
		return err
	}
	var layer v1.Layer
	switch parsedURL.Scheme {
	case "http", "https":
		resp, err := http.Get(tarURL)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("file not valid: status code: %d", resp.StatusCode)
		}
		layer, err = tarball.LayerFromReader(resp.Body)
		if err != nil {
			return err
		}
	case "file":
		layer, err = tarball.LayerFromFile(parsedURL.Path)
		if err != nil {
			return fmt.Errorf("file not valid: %w", err)
		}
	default:
		return fmt.Errorf("invalid url: %s", tarURL)
	}

	image, err = mutate.AppendLayers(image, layer)
	if err != nil {
		return err
	}

	if useDaemon {
		outputImageTag, err := name.NewTag(outputImageRef.Name())
		if err != nil {
			return err
		}

		if _, err = daemon.Write(outputImageTag, image); err != nil {
			return err
		}
	}
	if useRemote {
		if err = remote.Write(outputImageRef, image, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
			return err
		}
	}

	return nil
}
