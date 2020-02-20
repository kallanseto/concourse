package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Constants
const NAMESPACE = "flux"
const SERVICEACCOUNT = "flux"
const GITSECRET = "flux-git-auth"
const REPOIP = "10.51.4.163"
const REPOHOSTNAME = "tfs"
const REPONAME = "clingo"
const REPOWORKINGDIR = "/tmp/repo"
const REPODIR = REPOWORKINGDIR + "/" + REPONAME

// Project type
type Project struct {
	Cluster     string `json:"cluster"`
	Buildnumber string `json:"buildnumber"`
	Name        string `json:"name"`
	Owner       string `json:"owner`
	Team        string `json:"team"`
	Email       string `json:"email"`
	CPU         int    `json:"cpu"`
	Memory      int    `json:"memory"`
}

func jobCreateProject(c echo.Context) error {
	p := new(Project)
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

	jobsClient := clientset.BatchV1().Jobs(NAMESPACE)

	//	result, err := jobsClient.Create(context.TODO(), j, metav1.CreateOptions{})
	result, err := jobsClient.Create(j)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created job %q.\n", result.GetObjectMeta().GetName())

	return c.JSONPretty(http.StatusCreated, j, "  ")
}

func newJob(p *Project) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: p.Name + "-",
			Namespace:    NAMESPACE,
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
							Image:   "alpine/git",
							Command: []string{"clone"},
							Args: []string{
								"-c user.email=ocp-platform@test.com",
								"-c user.name=svcp_ocp_gitops",
								"https://$(GIT_AUTHUSER):$(GIT_AUTHKEY)github.com/kallanseto/clingo",
							},
							WorkingDir: REPOWORKINGDIR,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: GITSECRET,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: REPOWORKINGDIR,
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
							Image:   "alpine/git",
							Command: []string{"checkout"},
							Args: []string{
								"-b",
								p.Name + "-onboarding",
							},
							WorkingDir: REPODIR,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: REPOWORKINGDIR,
								},
							},
						},
						{
							Name:    "step-3-add-project",
							Image:   "kallanseto/clingo:0.1",
							Command: []string{"clingo create"},
							Args: []string{
								"--cluster=" + p.Cluster,
								"--buildnumber=" + p.Buildnumber,
								"--name=" + p.Name,
								"--owner=" + p.Owner,
								"--team=" + p.Team,
								"--email=" + p.Email,
								"--cpu=" + strconv.Itoa(p.CPU),
								"--memory=" + strconv.Itoa(p.Memory),
							},
							WorkingDir: REPODIR + "/test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: REPOWORKINGDIR,
								},
							},
						},
						{
							Name:    "step-4-add-files",
							Image:   "alpine/git",
							Command: []string{"add"},
							Args: []string{
								".",
							},
							WorkingDir: REPODIR,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: REPOWORKINGDIR,
								},
							},
						},
						{
							Name:    "step-5-commit-changes",
							Image:   "alpine/git",
							Command: []string{"commit"},
							Args: []string{
								"-am",
								p.Name + "-onboarding",
							},
							WorkingDir: REPODIR,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: REPOWORKINGDIR,
								},
							},
						},
						{
							Name:    "step-6-push-changes",
							Image:   "alpine/git",
							Command: []string{"push"},
							Args: []string{
								"-u",
								"origin",
								p.Name + "-onboarding",
							},
							WorkingDir: REPODIR,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: GITSECRET,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: REPOWORKINGDIR,
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
							IP:        REPOIP,
							Hostnames: []string{REPOHOSTNAME},
						},
					},
					ServiceAccountName: SERVICEACCOUNT,
					NodeSelector:       map[string]string{"node-role.kubernetes.io/infra": "true"},
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
