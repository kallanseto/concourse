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

// Namespace where we will deploy & run
const NAMESPACE = "flux"

// Project type
type Project struct {
	Name   string `json:"name"`
	Owner  string `json:"owner`
	Team   string `json:"team"`
	Email  string `json:"email"`
	CPU    int    `json:"cpu"`
	Memory int    `json:"memory"`
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
							Name:    "git-runner",
							Image:   "git-runner:1.0",
							Command: []string{"/bin/git"},
							Args:    []string{"clone", "-b", p.Name + "-onboarding", "https://github.com/kallanseto/namespaces"},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "onboard-cli",
							Image:   "onboard-cli:1.0",
							Command: []string{"/bin/onboard"},
							Args:    []string{"arg1", "arg2"},
							Env: []corev1.EnvVar{
								{
									Name:  "ONBOARD_NAME",
									Value: p.Name,
								},
								{
									Name:  "ONBOARD_OWNER",
									Value: p.Owner,
								},
								{
									Name:  "ONBOARD_TEAM",
									Value: p.Team,
								},
								{
									Name:  "ONBOARD_EMAIL",
									Value: p.Email,
								},
								{
									Name:  "ONBOARD_CPU",
									Value: strconv.Itoa(p.CPU),
								},
								{
									Name:  "ONBOARD_MEMORY",
									Value: strconv.Itoa(p.Memory),
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
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
