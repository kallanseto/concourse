package main

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants
const NAMESPACE = "flux"
const SERVICEACCOUNT = "flux"
const GITSECRET = "flux-git-auth"
const REPOIP = "10.51.4.163"
const REPOHOSTNAME = "tfs"

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

	// // creates the in-cluster config
	// config, err := rest.InClusterConfig()
	// if err != nil {
	// 	panic(err.Error())
	// }
	// // creates the clientset
	// clientset, err := kubernetes.NewForConfig(config)
	// if err != nil {
	// 	panic(err.Error())
	// }

	// jobsClient := clientset.BatchV1().Jobs(NAMESPACE)

	// result, err := jobsClient.Create(context.TODO(), j, metav1.CreateOptions{})
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Created job %q.\n", result.GetObjectMeta().GetName())

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
							Image:   "git-runner:1.0",
							Command: []string{"/bin/git"},
							Args: []string{
								"clone",
								"-b",
								p.Name + "-onboarding",
								"https://$(GIT_AUTHUSER):$(GIT_AUTHKEY)github.com/kallanseto/namespaces",
							},
							WorkingDir: "/tmp/repo",
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
									MountPath: "/tmp/repo",
								},
								{
									Name:      "certs",
									MountPath: "/tmp/certs",
									ReadOnly:  true,
								},
							},
						},
						{
							Name:    "step-2-add-project",
							Image:   "onboard-cli:1.0",
							Command: []string{"/bin/onboard"},
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
							WorkingDir: "/tmp/repo",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "repo",
									MountPath: "/tmp/repo",
								},
							},
						},
						{
							Name:    "step-3-commit-changes",
							Image:   "git-runner:1.0",
							Command: []string{"/bin/git"},
							Args: []string{
								"commit",
								"-am",
								p.Name + "-onboarding",
							},
							WorkingDir: "/tmp/repo",
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
									MountPath: "/tmp/repo",
								},
							},
						},
						{
							Name:    "step-4-push-changes",
							Image:   "git-runner:1.0",
							Command: []string{"/bin/git"},
							Args: []string{
								"push",
								"-u",
								"origin",
								p.Name + "-onboarding",
							},
							WorkingDir: "/tmp/repo",
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
									MountPath: "/tmp/repo",
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
							Image:   "onboard-cli:1.0",
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
