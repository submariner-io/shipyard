package deploy

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta "k8s.io/api/extensions/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1beta "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

// Resources deploys k8s resources
func Resources(clName string, clientSet kubernetes.Interface, deploymentFile string, resourceName string) error {
	acceptedK8sTypes := regexp.MustCompile(`(Role|RoleBinding|ClusterRole|ClusterRoleBinding|ServiceAccount|ConfigMap|DaemonSet|Deployment|Service|Pod)`)
	fileAsString := deploymentFile[:]
	sepYamlfiles := strings.Split(fileAsString, "---")
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode([]byte(f), nil, nil)

		if err != nil {
			return errors.Wrap(err, "Error while decoding YAML object. Err was: ")
		}

		if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
			log.Warnf("The file contains K8s object types which are not supported! Skipping object with type: %s", groupVersionKind.Kind)
		} else {
			switch o := obj.(type) {
			case *corev1.ServiceAccount:
				result, err := clientSet.CoreV1().ServiceAccounts(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ ServiceAccount %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *rbacv1.Role:
				result, err := clientSet.RbacV1().Roles(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ Role %s created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *rbacv1.RoleBinding:
				result, err := clientSet.RbacV1().RoleBindings(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ RoleBinding %s created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *rbacv1.ClusterRole:
				result, err := clientSet.RbacV1().ClusterRoles().Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ ClusterRole %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *rbacv1.ClusterRoleBinding:
				result, err := clientSet.RbacV1().ClusterRoleBindings().Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ ClusterRoleBinding %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *corev1.ConfigMap:
				result, err := clientSet.CoreV1().ConfigMaps(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ ConfigMap %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *corev1.Service:
				result, err := clientSet.CoreV1().Services(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ Service %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *corev1.Pod:
				result, err := clientSet.CoreV1().Pods(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ Pod %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *policyv1beta1.PodSecurityPolicy:
				result, err := clientSet.PolicyV1beta1().PodSecurityPolicies().Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ PodSecurityPolicy %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *appsv1.DaemonSet:
				result, err := clientSet.AppsV1().DaemonSets(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ Daemonset %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *extv1beta.DaemonSet:
				result, err := clientSet.ExtensionsV1beta1().DaemonSets(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ Daemonset %s was created for %s at: %s.", o.Name, clName, result.CreationTimestamp)
				}
			case *appsv1.Deployment:
				result, err := clientSet.AppsV1().Deployments(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ Deployment %s was created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			case *extv1beta.Deployment:
				result, err := clientSet.ExtensionsV1beta1().Deployments(o.Namespace).Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ Deployment %s created for %s at: %s", o.Name, clName, result.CreationTimestamp)
				}
			}
		}
	}
	log.Debugf("✔ %s resources were deployed to %s.", resourceName, clName)
	return nil
}

// CrdResources deploys k8s CRD resources
func CrdResources(clName string, apiExtClientSet apiextclientset.Interface, deploymentFile string) error {
	acceptedK8sTypes := regexp.MustCompile(`(CustomResourceDefinition)`)
	fileAsString := deploymentFile[:]
	sepYamlfiles := strings.Split(fileAsString, "---")
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		decode := apiextscheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode([]byte(f), nil, nil)

		if err != nil {
			return errors.Wrap(err, "Error while decoding YAML object. Err was: ")
		}

		if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
			log.Warnf("The file contains K8s object types which are not supported! Skipping object with type: %s", groupVersionKind.Kind)
		} else {
			switch o := obj.(type) {
			case *apiextv1beta.CustomResourceDefinition:
				_, err := apiExtClientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Create(o)
				if err != nil && !apierr.IsAlreadyExists(err) {
					return err
				} else if err == nil {
					log.Debugf("✔ CRD %s was created for %s.", o.Name, clName)
				}
			}
		}
	}
	return nil
}
