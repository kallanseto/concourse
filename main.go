package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	clingo "clingo/cmd"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Global environment variables
var cluster = os.Getenv("CLUSTER")
var buildNumber = os.Getenv("BUILDNUMBER")
var namespace = os.Getenv("NAMESPACE")
var gitRepo = os.Getenv("GIT_REPO")
var gitName = os.Getenv("GIT_NAME")
var gitEmail = os.Getenv("GIT_EMAIL")
var gitSecret = os.Getenv("GIT_SECRET")
var gitIP = os.Getenv("GIT_IP")
var gitHostname = os.Getenv("GIT_HOSTNAME")
var gitClientImage = os.Getenv("GIT_CLIENT_IMAGE")
var repoName = os.Getenv("REPO_NAME")
var repoWorkingDir = os.Getenv("REPO_WORKINGDIR")
var clingoImage = os.Getenv("CLINGO_IMAGE")
var clingoBaseDir = os.Getenv("CLINGO_BASEDIR")

func jobCreateProject(c echo.Context) error {
	p := new(clingo.Project)
	if err := c.Bind(p); err != nil {
		return err
	}

	j := newJob(p)

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	jobsClient := clientset.BatchV1().Jobs(namespace)

	result, err := jobsClient.Create(context.TODO(), j, metav1.CreateOptions{})
	//result, err := jobsClient.Create(j)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created job %q.\n", result.GetObjectMeta().GetName())

	return c.JSONPretty(http.StatusCreated, result.GetObjectMeta().GetName(), "  ")
}

func newJob(p *clingo.Project) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: p.Name + "-",
			Namespace:    namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: p.Name + "-",
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:    "step-1-clone-repo",
							Image:   gitClientImage,
							Command: []string{"git"},
							Args: []string{
								"clone",
								"-c user.email=" + gitEmail,
								"-c user.name=" + gitName,
								"https://$(GIT_AUTHUSER):$(GIT_AUTHKEY)@" + gitRepo,
							},
							WorkingDir: repoWorkingDir,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: gitSecret,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: repoWorkingDir,
								},
								{
									Name:      "certs",
									MountPath: "/tmp/certs",
									ReadOnly:  true,
								},
							},
						},
						{
							Name:    "step-2-checkout-branch",
							Image:   gitClientImage,
							Command: []string{"git"},
							Args: []string{
								"checkout",
								"-b",
								p.Name + "-onboarding",
							},
							WorkingDir: repoWorkingDir + "/" + repoName,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: repoWorkingDir,
								},
							},
						},
						{
							Name:    "step-3-add-project",
							Image:   clingoImage,
							Command: []string{"clingo"},
							Args: []string{
								"create",
								"--cluster=" + cluster,
								"--buildnumber=" + buildNumber,
								"--name=" + p.Name,
								"--owner=" + p.Owner,
								"--team=" + p.Team,
								"--email=" + p.Email,
								"--service=" + p.Service,
								"--application=" + p.Application,
								"--domain=" + p.Domain,
								"--namespacevip=" + p.Namespacevip,
								"--snatip=" + p.Snatip,
								"--cpu=" + strconv.Itoa(p.CPU),
								"--memory=" + strconv.Itoa(p.Memory),
							},
							WorkingDir: clingoBaseDir,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: repoWorkingDir,
								},
							},
						},
						{
							Name:    "step-4-add-files",
							Image:   gitClientImage,
							Command: []string{"git"},
							Args: []string{
								"add",
								".",
							},
							WorkingDir: repoWorkingDir + "/" + repoName,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: repoWorkingDir,
								},
							},
						},
						{
							Name:    "step-5-commit-changes",
							Image:   gitClientImage,
							Command: []string{"git"},
							Args: []string{
								"commit",
								"-am",
								p.Name + "-onboarding",
							},
							WorkingDir: repoWorkingDir + "/" + repoName,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: repoWorkingDir,
								},
							},
						},
						{
							Name:    "step-6-push-changes",
							Image:   gitClientImage,
							Command: []string{"git"},
							Args: []string{
								"push",
								"-u",
								"origin",
								p.Name + "-onboarding",
							},
							WorkingDir: repoWorkingDir + "/" + repoName,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: gitSecret,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: repoWorkingDir,
								},
								{
									Name:      "certs",
									MountPath: "/tmp/certs",
									ReadOnly:  true,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "job-complete",
							Image:   "busybox",
							Command: []string{"echo"},
							Args:    []string{"job completed"},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
					HostAliases: []corev1.HostAlias{
						{
							IP:        gitIP,
							Hostnames: []string{gitHostname},
						},
					},
					NodeSelector: map[string]string{"node-role.kubernetes.io/infra": "true"},
					Volumes: []corev1.Volume{
						{
							Name: "repo",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "certs",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "rootca",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	e.POST("/project", jobCreateProject)
	// e.GET("/project/:id", getProject)
	// e.PUT("/project/:id", updateProject)
	// e.DELETE("/project/:id", deleteProject)

	e.Logger.Fatal(e.Start(":8080"))
}
