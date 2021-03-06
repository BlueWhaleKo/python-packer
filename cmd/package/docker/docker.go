/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BlueWhaleKo/python-packer/pkg/util"
	docker "github.com/BlueWhaleKo/python-packer/pkg/util/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker --[flags] [options]",
	Short: "pack python projects into a zip file",
	Long:  "pack python projects into a zip file",
	Run: func(cmd *cobra.Command, args []string) {
		main(cmd, args)
	},
}

func NewDockerCommand() *cobra.Command {
	return dockerCmd
}

type dockerArgs struct {
	ProjectPath    string
	OutputImage    string
	BaseImage      string
	DockerfilePath string
}

var dockerargs = &dockerArgs{}

func init() {
	// parse args
	dockerCmd.Flags().StringVar(&dockerargs.ProjectPath, "project-path", "", "(required) path to python project directory")
	dockerCmd.Flags().StringVar(&dockerargs.DockerfilePath, "dockerfile", "", "path to Dockerfile")
	dockerCmd.Flags().StringVar(&dockerargs.BaseImage, "base-image", "", "(required) name of base image to build from")
	dockerCmd.Flags().StringVar(&dockerargs.OutputImage, "output-image", "", "(required) name of output image")

	dockerCmd.MarkFlagRequired("project-path")
	dockerCmd.MarkFlagRequired("output-image")
	dockerCmd.MarkFlagRequired("base-image")
}

func validate() error {
	if dockerargs.DockerfilePath == "" {
		dockerargs.DockerfilePath = filepath.Join(dockerargs.ProjectPath, "Dockerfile")
		logrus.Warnf("--dockerfile is not specified. Use %s by default", dockerargs.DockerfilePath)
	}

	logrus.Info("Project: ", dockerargs.ProjectPath)
	logrus.Info("Dockerfile: ", dockerargs.DockerfilePath)
	logrus.Info("Base Image: ", dockerargs.BaseImage)
	logrus.Info("Output Image: ", dockerargs.OutputImage)

	if !util.FileExists(filepath.Join(dockerargs.ProjectPath, "__main__.py")) {
		return fmt.Errorf("You need __main__.py at python project root %s as entrypoint", dockerargs.ProjectPath)
	}
	filepath.Join(dockerargs.ProjectPath, "Dockerfile")

	return nil
}

func createDockerfile() *docker.Dockerfile {
	stage := "builder"
	targetDir := "/app"

	// build stage
	d := docker.NewDockerfile()
	d.FromAs("python", stage)
	d.Run("pip", "install", "pipreqs")
	d.Workdir(targetDir)
	d.Add(".", ".")
	d.Run("pipreqs", ".")
	d.Run("pip", "install", "-r", "./requirements.txt", "-t", ".")

	// runtime stage
	d.From(dockerargs.BaseImage)
	d.CopyFrom(targetDir, targetDir, stage)
	d.Workdir("/")
	d.Entrypoint("python", targetDir)

	return d
}

func main(cmd *cobra.Command, args []string) {
	err := validate()
	if err != nil {
		logrus.Fatal(err)
	}

	if !util.FileExists(dockerargs.DockerfilePath) {
		logrus.Warnf("Dockerfile not found at %s. Create a default", dockerargs.DockerfilePath)

		contents := createDockerfile().Build()
		logrus.Info("Dockerfile:\n", contents)
		err := util.Write(dockerargs.DockerfilePath, contents)
		if err != nil {
			logrus.Fatal(err)
		}

		defer os.Remove(dockerargs.DockerfilePath)
	}

	// create client
	logrus.Info("Create docker client")
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Build docker image")
	res, err := docker.BuildImageFromPath(c, dockerargs.ProjectPath, types.ImageBuildOptions{})
	if err != nil {
		logrus.Fatal(err)
	}

	docker.Print(res.Body)
	logrus.Infof("Successfuly built docker image '%s'", dockerargs.OutputImage)
}
