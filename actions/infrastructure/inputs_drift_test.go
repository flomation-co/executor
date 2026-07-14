// Cross-action invariants for the Infrastructure ▸ Kubernetes and ▸ Helm nodes.
//
// Every one of these 78 actions re-declares the eight-field credential block
// inline, because the manifest generator AST-parses the Inputs literal and cannot
// see through a shared variable (see kubernetes.AuthInputs). That makes
// kubernetes.AuthInputs documentation rather than enforcement — so this test does
// the enforcing: a copy that drifts fails CI with the offending action named.
//
// It also pins three properties that are invisible at the call site and expensive
// to notice later:
//
//   - a destructive action carries its confirm_destructive guard LAST and required;
//   - every Icon resolves to a glyph the editor actually ships (an unknown badge
//     silently renders as "?");
//   - every action reports success/error/tool_result, which the flow engine and the
//     AI tool loop both depend on.
//
// SCOPE. Infrastructure now holds a third node — AAP / AWX — whose credential
// block, icon base and destructive set are its own. Its invariants are enforced by
// a sibling file in this same package (awx_inputs_drift_test.go), on the
// per-node-sibling precedent of actions/opentofu/inputs_drift_test.go. The
// per-node tests below therefore scope themselves with scopedToThisFile(), while
// TestEveryRegisteredActionIsCovered deliberately does NOT: it unions the tables
// of every file in the package, so an action registered under infrastructure/ that
// no file covers still fails, whichever node it belongs to.
package infrastructure_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions"
	"flomation.app/automate/executor/actions/infrastructure/helm"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"

	hlm_chart_lint "flomation.app/automate/executor/actions/infrastructure/helm/chart_lint"
	hlm_chart_show "flomation.app/automate/executor/actions/infrastructure/helm/chart_show"
	hlm_chart_template "flomation.app/automate/executor/actions/infrastructure/helm/chart_template"
	hlm_release_get "flomation.app/automate/executor/actions/infrastructure/helm/release_get"
	hlm_release_history "flomation.app/automate/executor/actions/infrastructure/helm/release_history"
	hlm_release_install "flomation.app/automate/executor/actions/infrastructure/helm/release_install"
	hlm_release_list "flomation.app/automate/executor/actions/infrastructure/helm/release_list"
	hlm_release_rollback "flomation.app/automate/executor/actions/infrastructure/helm/release_rollback"
	hlm_release_status "flomation.app/automate/executor/actions/infrastructure/helm/release_status"
	hlm_release_test "flomation.app/automate/executor/actions/infrastructure/helm/release_test"
	hlm_release_uninstall "flomation.app/automate/executor/actions/infrastructure/helm/release_uninstall"
	hlm_release_upgrade "flomation.app/automate/executor/actions/infrastructure/helm/release_upgrade"
	k8s_apply_manifest "flomation.app/automate/executor/actions/infrastructure/kubernetes/apply_manifest"
	k8s_configmap_create "flomation.app/automate/executor/actions/infrastructure/kubernetes/configmap_create"
	k8s_configmap_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/configmap_delete"
	k8s_configmap_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/configmap_get"
	k8s_configmap_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/configmap_list"
	k8s_configmap_update "flomation.app/automate/executor/actions/infrastructure/kubernetes/configmap_update"
	k8s_cronjob_create "flomation.app/automate/executor/actions/infrastructure/kubernetes/cronjob_create"
	k8s_cronjob_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/cronjob_delete"
	k8s_cronjob_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/cronjob_get"
	k8s_cronjob_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/cronjob_list"
	k8s_cronjob_resume "flomation.app/automate/executor/actions/infrastructure/kubernetes/cronjob_resume"
	k8s_cronjob_suspend "flomation.app/automate/executor/actions/infrastructure/kubernetes/cronjob_suspend"
	k8s_cronjob_trigger "flomation.app/automate/executor/actions/infrastructure/kubernetes/cronjob_trigger"
	k8s_daemonset_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/daemonset_delete"
	k8s_daemonset_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/daemonset_get"
	k8s_daemonset_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/daemonset_list"
	k8s_daemonset_restart "flomation.app/automate/executor/actions/infrastructure/kubernetes/daemonset_restart"
	k8s_deployment_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/deployment_delete"
	k8s_deployment_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/deployment_get"
	k8s_deployment_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/deployment_list"
	k8s_deployment_restart "flomation.app/automate/executor/actions/infrastructure/kubernetes/deployment_restart"
	k8s_deployment_rollout_status "flomation.app/automate/executor/actions/infrastructure/kubernetes/deployment_rollout_status"
	k8s_deployment_scale "flomation.app/automate/executor/actions/infrastructure/kubernetes/deployment_scale"
	k8s_event_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/event_list"
	k8s_hpa_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/hpa_delete"
	k8s_hpa_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/hpa_get"
	k8s_hpa_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/hpa_list"
	k8s_ingress_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/ingress_delete"
	k8s_ingress_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/ingress_get"
	k8s_ingress_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/ingress_list"
	k8s_job_create "flomation.app/automate/executor/actions/infrastructure/kubernetes/job_create"
	k8s_job_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/job_delete"
	k8s_job_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/job_get"
	k8s_job_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/job_list"
	k8s_namespace_create "flomation.app/automate/executor/actions/infrastructure/kubernetes/namespace_create"
	k8s_namespace_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/namespace_delete"
	k8s_namespace_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/namespace_get"
	k8s_namespace_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/namespace_list"
	k8s_node_cordon "flomation.app/automate/executor/actions/infrastructure/kubernetes/node_cordon"
	k8s_node_drain "flomation.app/automate/executor/actions/infrastructure/kubernetes/node_drain"
	k8s_node_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/node_get"
	k8s_node_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/node_list"
	k8s_node_uncordon "flomation.app/automate/executor/actions/infrastructure/kubernetes/node_uncordon"
	k8s_pod_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/pod_delete"
	k8s_pod_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/pod_get"
	k8s_pod_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/pod_list"
	k8s_pod_logs "flomation.app/automate/executor/actions/infrastructure/kubernetes/pod_logs"
	k8s_pvc_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/pvc_delete"
	k8s_pvc_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/pvc_get"
	k8s_pvc_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/pvc_list"
	k8s_secret_create "flomation.app/automate/executor/actions/infrastructure/kubernetes/secret_create"
	k8s_secret_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/secret_delete"
	k8s_secret_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/secret_get"
	k8s_secret_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/secret_list"
	k8s_service_create "flomation.app/automate/executor/actions/infrastructure/kubernetes/service_create"
	k8s_service_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/service_delete"
	k8s_service_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/service_get"
	k8s_service_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/service_list"
	k8s_serviceaccount_create "flomation.app/automate/executor/actions/infrastructure/kubernetes/serviceaccount_create"
	k8s_serviceaccount_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/serviceaccount_delete"
	k8s_serviceaccount_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/serviceaccount_get"
	k8s_serviceaccount_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/serviceaccount_list"
	k8s_statefulset_delete "flomation.app/automate/executor/actions/infrastructure/kubernetes/statefulset_delete"
	k8s_statefulset_get "flomation.app/automate/executor/actions/infrastructure/kubernetes/statefulset_get"
	k8s_statefulset_list "flomation.app/automate/executor/actions/infrastructure/kubernetes/statefulset_list"
	k8s_statefulset_restart "flomation.app/automate/executor/actions/infrastructure/kubernetes/statefulset_restart"
	k8s_statefulset_scale "flomation.app/automate/executor/actions/infrastructure/kubernetes/statefulset_scale"
)

func actionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"kubernetes/apply_manifest":            k8s_apply_manifest.Inputs[:],
		"kubernetes/configmap_create":          k8s_configmap_create.Inputs[:],
		"kubernetes/configmap_delete":          k8s_configmap_delete.Inputs[:],
		"kubernetes/configmap_get":             k8s_configmap_get.Inputs[:],
		"kubernetes/configmap_list":            k8s_configmap_list.Inputs[:],
		"kubernetes/configmap_update":          k8s_configmap_update.Inputs[:],
		"kubernetes/cronjob_create":            k8s_cronjob_create.Inputs[:],
		"kubernetes/cronjob_delete":            k8s_cronjob_delete.Inputs[:],
		"kubernetes/cronjob_get":               k8s_cronjob_get.Inputs[:],
		"kubernetes/cronjob_list":              k8s_cronjob_list.Inputs[:],
		"kubernetes/cronjob_resume":            k8s_cronjob_resume.Inputs[:],
		"kubernetes/cronjob_suspend":           k8s_cronjob_suspend.Inputs[:],
		"kubernetes/cronjob_trigger":           k8s_cronjob_trigger.Inputs[:],
		"kubernetes/daemonset_delete":          k8s_daemonset_delete.Inputs[:],
		"kubernetes/daemonset_get":             k8s_daemonset_get.Inputs[:],
		"kubernetes/daemonset_list":            k8s_daemonset_list.Inputs[:],
		"kubernetes/daemonset_restart":         k8s_daemonset_restart.Inputs[:],
		"kubernetes/deployment_delete":         k8s_deployment_delete.Inputs[:],
		"kubernetes/deployment_get":            k8s_deployment_get.Inputs[:],
		"kubernetes/deployment_list":           k8s_deployment_list.Inputs[:],
		"kubernetes/deployment_restart":        k8s_deployment_restart.Inputs[:],
		"kubernetes/deployment_rollout_status": k8s_deployment_rollout_status.Inputs[:],
		"kubernetes/deployment_scale":          k8s_deployment_scale.Inputs[:],
		"kubernetes/event_list":                k8s_event_list.Inputs[:],
		"kubernetes/hpa_delete":                k8s_hpa_delete.Inputs[:],
		"kubernetes/hpa_get":                   k8s_hpa_get.Inputs[:],
		"kubernetes/hpa_list":                  k8s_hpa_list.Inputs[:],
		"kubernetes/ingress_delete":            k8s_ingress_delete.Inputs[:],
		"kubernetes/ingress_get":               k8s_ingress_get.Inputs[:],
		"kubernetes/ingress_list":              k8s_ingress_list.Inputs[:],
		"kubernetes/job_create":                k8s_job_create.Inputs[:],
		"kubernetes/job_delete":                k8s_job_delete.Inputs[:],
		"kubernetes/job_get":                   k8s_job_get.Inputs[:],
		"kubernetes/job_list":                  k8s_job_list.Inputs[:],
		"kubernetes/namespace_create":          k8s_namespace_create.Inputs[:],
		"kubernetes/namespace_delete":          k8s_namespace_delete.Inputs[:],
		"kubernetes/namespace_get":             k8s_namespace_get.Inputs[:],
		"kubernetes/namespace_list":            k8s_namespace_list.Inputs[:],
		"kubernetes/node_cordon":               k8s_node_cordon.Inputs[:],
		"kubernetes/node_drain":                k8s_node_drain.Inputs[:],
		"kubernetes/node_get":                  k8s_node_get.Inputs[:],
		"kubernetes/node_list":                 k8s_node_list.Inputs[:],
		"kubernetes/node_uncordon":             k8s_node_uncordon.Inputs[:],
		"kubernetes/pod_delete":                k8s_pod_delete.Inputs[:],
		"kubernetes/pod_get":                   k8s_pod_get.Inputs[:],
		"kubernetes/pod_list":                  k8s_pod_list.Inputs[:],
		"kubernetes/pod_logs":                  k8s_pod_logs.Inputs[:],
		"kubernetes/pvc_delete":                k8s_pvc_delete.Inputs[:],
		"kubernetes/pvc_get":                   k8s_pvc_get.Inputs[:],
		"kubernetes/pvc_list":                  k8s_pvc_list.Inputs[:],
		"kubernetes/secret_create":             k8s_secret_create.Inputs[:],
		"kubernetes/secret_delete":             k8s_secret_delete.Inputs[:],
		"kubernetes/secret_get":                k8s_secret_get.Inputs[:],
		"kubernetes/secret_list":               k8s_secret_list.Inputs[:],
		"kubernetes/service_create":            k8s_service_create.Inputs[:],
		"kubernetes/service_delete":            k8s_service_delete.Inputs[:],
		"kubernetes/service_get":               k8s_service_get.Inputs[:],
		"kubernetes/service_list":              k8s_service_list.Inputs[:],
		"kubernetes/serviceaccount_create":     k8s_serviceaccount_create.Inputs[:],
		"kubernetes/serviceaccount_delete":     k8s_serviceaccount_delete.Inputs[:],
		"kubernetes/serviceaccount_get":        k8s_serviceaccount_get.Inputs[:],
		"kubernetes/serviceaccount_list":       k8s_serviceaccount_list.Inputs[:],
		"kubernetes/statefulset_delete":        k8s_statefulset_delete.Inputs[:],
		"kubernetes/statefulset_get":           k8s_statefulset_get.Inputs[:],
		"kubernetes/statefulset_list":          k8s_statefulset_list.Inputs[:],
		"kubernetes/statefulset_restart":       k8s_statefulset_restart.Inputs[:],
		"kubernetes/statefulset_scale":         k8s_statefulset_scale.Inputs[:],
		"helm/chart_lint":                      hlm_chart_lint.Inputs[:],
		"helm/chart_show":                      hlm_chart_show.Inputs[:],
		"helm/chart_template":                  hlm_chart_template.Inputs[:],
		"helm/release_get":                     hlm_release_get.Inputs[:],
		"helm/release_history":                 hlm_release_history.Inputs[:],
		"helm/release_install":                 hlm_release_install.Inputs[:],
		"helm/release_list":                    hlm_release_list.Inputs[:],
		"helm/release_rollback":                hlm_release_rollback.Inputs[:],
		"helm/release_status":                  hlm_release_status.Inputs[:],
		"helm/release_test":                    hlm_release_test.Inputs[:],
		"helm/release_uninstall":               hlm_release_uninstall.Inputs[:],
		"helm/release_upgrade":                 hlm_release_upgrade.Inputs[:],
	}
}

func actionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"kubernetes/apply_manifest":            k8s_apply_manifest.Outputs[:],
		"kubernetes/configmap_create":          k8s_configmap_create.Outputs[:],
		"kubernetes/configmap_delete":          k8s_configmap_delete.Outputs[:],
		"kubernetes/configmap_get":             k8s_configmap_get.Outputs[:],
		"kubernetes/configmap_list":            k8s_configmap_list.Outputs[:],
		"kubernetes/configmap_update":          k8s_configmap_update.Outputs[:],
		"kubernetes/cronjob_create":            k8s_cronjob_create.Outputs[:],
		"kubernetes/cronjob_delete":            k8s_cronjob_delete.Outputs[:],
		"kubernetes/cronjob_get":               k8s_cronjob_get.Outputs[:],
		"kubernetes/cronjob_list":              k8s_cronjob_list.Outputs[:],
		"kubernetes/cronjob_resume":            k8s_cronjob_resume.Outputs[:],
		"kubernetes/cronjob_suspend":           k8s_cronjob_suspend.Outputs[:],
		"kubernetes/cronjob_trigger":           k8s_cronjob_trigger.Outputs[:],
		"kubernetes/daemonset_delete":          k8s_daemonset_delete.Outputs[:],
		"kubernetes/daemonset_get":             k8s_daemonset_get.Outputs[:],
		"kubernetes/daemonset_list":            k8s_daemonset_list.Outputs[:],
		"kubernetes/daemonset_restart":         k8s_daemonset_restart.Outputs[:],
		"kubernetes/deployment_delete":         k8s_deployment_delete.Outputs[:],
		"kubernetes/deployment_get":            k8s_deployment_get.Outputs[:],
		"kubernetes/deployment_list":           k8s_deployment_list.Outputs[:],
		"kubernetes/deployment_restart":        k8s_deployment_restart.Outputs[:],
		"kubernetes/deployment_rollout_status": k8s_deployment_rollout_status.Outputs[:],
		"kubernetes/deployment_scale":          k8s_deployment_scale.Outputs[:],
		"kubernetes/event_list":                k8s_event_list.Outputs[:],
		"kubernetes/hpa_delete":                k8s_hpa_delete.Outputs[:],
		"kubernetes/hpa_get":                   k8s_hpa_get.Outputs[:],
		"kubernetes/hpa_list":                  k8s_hpa_list.Outputs[:],
		"kubernetes/ingress_delete":            k8s_ingress_delete.Outputs[:],
		"kubernetes/ingress_get":               k8s_ingress_get.Outputs[:],
		"kubernetes/ingress_list":              k8s_ingress_list.Outputs[:],
		"kubernetes/job_create":                k8s_job_create.Outputs[:],
		"kubernetes/job_delete":                k8s_job_delete.Outputs[:],
		"kubernetes/job_get":                   k8s_job_get.Outputs[:],
		"kubernetes/job_list":                  k8s_job_list.Outputs[:],
		"kubernetes/namespace_create":          k8s_namespace_create.Outputs[:],
		"kubernetes/namespace_delete":          k8s_namespace_delete.Outputs[:],
		"kubernetes/namespace_get":             k8s_namespace_get.Outputs[:],
		"kubernetes/namespace_list":            k8s_namespace_list.Outputs[:],
		"kubernetes/node_cordon":               k8s_node_cordon.Outputs[:],
		"kubernetes/node_drain":                k8s_node_drain.Outputs[:],
		"kubernetes/node_get":                  k8s_node_get.Outputs[:],
		"kubernetes/node_list":                 k8s_node_list.Outputs[:],
		"kubernetes/node_uncordon":             k8s_node_uncordon.Outputs[:],
		"kubernetes/pod_delete":                k8s_pod_delete.Outputs[:],
		"kubernetes/pod_get":                   k8s_pod_get.Outputs[:],
		"kubernetes/pod_list":                  k8s_pod_list.Outputs[:],
		"kubernetes/pod_logs":                  k8s_pod_logs.Outputs[:],
		"kubernetes/pvc_delete":                k8s_pvc_delete.Outputs[:],
		"kubernetes/pvc_get":                   k8s_pvc_get.Outputs[:],
		"kubernetes/pvc_list":                  k8s_pvc_list.Outputs[:],
		"kubernetes/secret_create":             k8s_secret_create.Outputs[:],
		"kubernetes/secret_delete":             k8s_secret_delete.Outputs[:],
		"kubernetes/secret_get":                k8s_secret_get.Outputs[:],
		"kubernetes/secret_list":               k8s_secret_list.Outputs[:],
		"kubernetes/service_create":            k8s_service_create.Outputs[:],
		"kubernetes/service_delete":            k8s_service_delete.Outputs[:],
		"kubernetes/service_get":               k8s_service_get.Outputs[:],
		"kubernetes/service_list":              k8s_service_list.Outputs[:],
		"kubernetes/serviceaccount_create":     k8s_serviceaccount_create.Outputs[:],
		"kubernetes/serviceaccount_delete":     k8s_serviceaccount_delete.Outputs[:],
		"kubernetes/serviceaccount_get":        k8s_serviceaccount_get.Outputs[:],
		"kubernetes/serviceaccount_list":       k8s_serviceaccount_list.Outputs[:],
		"kubernetes/statefulset_delete":        k8s_statefulset_delete.Outputs[:],
		"kubernetes/statefulset_get":           k8s_statefulset_get.Outputs[:],
		"kubernetes/statefulset_list":          k8s_statefulset_list.Outputs[:],
		"kubernetes/statefulset_restart":       k8s_statefulset_restart.Outputs[:],
		"kubernetes/statefulset_scale":         k8s_statefulset_scale.Outputs[:],
		"helm/chart_lint":                      hlm_chart_lint.Outputs[:],
		"helm/chart_show":                      hlm_chart_show.Outputs[:],
		"helm/chart_template":                  hlm_chart_template.Outputs[:],
		"helm/release_get":                     hlm_release_get.Outputs[:],
		"helm/release_history":                 hlm_release_history.Outputs[:],
		"helm/release_install":                 hlm_release_install.Outputs[:],
		"helm/release_list":                    hlm_release_list.Outputs[:],
		"helm/release_rollback":                hlm_release_rollback.Outputs[:],
		"helm/release_status":                  hlm_release_status.Outputs[:],
		"helm/release_test":                    hlm_release_test.Outputs[:],
		"helm/release_uninstall":               hlm_release_uninstall.Outputs[:],
		"helm/release_upgrade":                 hlm_release_upgrade.Outputs[:],
	}
}

func actionIcons() map[string]string {
	return map[string]string{
		"kubernetes/apply_manifest":            k8s_apply_manifest.Icon,
		"kubernetes/configmap_create":          k8s_configmap_create.Icon,
		"kubernetes/configmap_delete":          k8s_configmap_delete.Icon,
		"kubernetes/configmap_get":             k8s_configmap_get.Icon,
		"kubernetes/configmap_list":            k8s_configmap_list.Icon,
		"kubernetes/configmap_update":          k8s_configmap_update.Icon,
		"kubernetes/cronjob_create":            k8s_cronjob_create.Icon,
		"kubernetes/cronjob_delete":            k8s_cronjob_delete.Icon,
		"kubernetes/cronjob_get":               k8s_cronjob_get.Icon,
		"kubernetes/cronjob_list":              k8s_cronjob_list.Icon,
		"kubernetes/cronjob_resume":            k8s_cronjob_resume.Icon,
		"kubernetes/cronjob_suspend":           k8s_cronjob_suspend.Icon,
		"kubernetes/cronjob_trigger":           k8s_cronjob_trigger.Icon,
		"kubernetes/daemonset_delete":          k8s_daemonset_delete.Icon,
		"kubernetes/daemonset_get":             k8s_daemonset_get.Icon,
		"kubernetes/daemonset_list":            k8s_daemonset_list.Icon,
		"kubernetes/daemonset_restart":         k8s_daemonset_restart.Icon,
		"kubernetes/deployment_delete":         k8s_deployment_delete.Icon,
		"kubernetes/deployment_get":            k8s_deployment_get.Icon,
		"kubernetes/deployment_list":           k8s_deployment_list.Icon,
		"kubernetes/deployment_restart":        k8s_deployment_restart.Icon,
		"kubernetes/deployment_rollout_status": k8s_deployment_rollout_status.Icon,
		"kubernetes/deployment_scale":          k8s_deployment_scale.Icon,
		"kubernetes/event_list":                k8s_event_list.Icon,
		"kubernetes/hpa_delete":                k8s_hpa_delete.Icon,
		"kubernetes/hpa_get":                   k8s_hpa_get.Icon,
		"kubernetes/hpa_list":                  k8s_hpa_list.Icon,
		"kubernetes/ingress_delete":            k8s_ingress_delete.Icon,
		"kubernetes/ingress_get":               k8s_ingress_get.Icon,
		"kubernetes/ingress_list":              k8s_ingress_list.Icon,
		"kubernetes/job_create":                k8s_job_create.Icon,
		"kubernetes/job_delete":                k8s_job_delete.Icon,
		"kubernetes/job_get":                   k8s_job_get.Icon,
		"kubernetes/job_list":                  k8s_job_list.Icon,
		"kubernetes/namespace_create":          k8s_namespace_create.Icon,
		"kubernetes/namespace_delete":          k8s_namespace_delete.Icon,
		"kubernetes/namespace_get":             k8s_namespace_get.Icon,
		"kubernetes/namespace_list":            k8s_namespace_list.Icon,
		"kubernetes/node_cordon":               k8s_node_cordon.Icon,
		"kubernetes/node_drain":                k8s_node_drain.Icon,
		"kubernetes/node_get":                  k8s_node_get.Icon,
		"kubernetes/node_list":                 k8s_node_list.Icon,
		"kubernetes/node_uncordon":             k8s_node_uncordon.Icon,
		"kubernetes/pod_delete":                k8s_pod_delete.Icon,
		"kubernetes/pod_get":                   k8s_pod_get.Icon,
		"kubernetes/pod_list":                  k8s_pod_list.Icon,
		"kubernetes/pod_logs":                  k8s_pod_logs.Icon,
		"kubernetes/pvc_delete":                k8s_pvc_delete.Icon,
		"kubernetes/pvc_get":                   k8s_pvc_get.Icon,
		"kubernetes/pvc_list":                  k8s_pvc_list.Icon,
		"kubernetes/secret_create":             k8s_secret_create.Icon,
		"kubernetes/secret_delete":             k8s_secret_delete.Icon,
		"kubernetes/secret_get":                k8s_secret_get.Icon,
		"kubernetes/secret_list":               k8s_secret_list.Icon,
		"kubernetes/service_create":            k8s_service_create.Icon,
		"kubernetes/service_delete":            k8s_service_delete.Icon,
		"kubernetes/service_get":               k8s_service_get.Icon,
		"kubernetes/service_list":              k8s_service_list.Icon,
		"kubernetes/serviceaccount_create":     k8s_serviceaccount_create.Icon,
		"kubernetes/serviceaccount_delete":     k8s_serviceaccount_delete.Icon,
		"kubernetes/serviceaccount_get":        k8s_serviceaccount_get.Icon,
		"kubernetes/serviceaccount_list":       k8s_serviceaccount_list.Icon,
		"kubernetes/statefulset_delete":        k8s_statefulset_delete.Icon,
		"kubernetes/statefulset_get":           k8s_statefulset_get.Icon,
		"kubernetes/statefulset_list":          k8s_statefulset_list.Icon,
		"kubernetes/statefulset_restart":       k8s_statefulset_restart.Icon,
		"kubernetes/statefulset_scale":         k8s_statefulset_scale.Icon,
		"helm/chart_lint":                      hlm_chart_lint.Icon,
		"helm/chart_show":                      hlm_chart_show.Icon,
		"helm/chart_template":                  hlm_chart_template.Icon,
		"helm/release_get":                     hlm_release_get.Icon,
		"helm/release_history":                 hlm_release_history.Icon,
		"helm/release_install":                 hlm_release_install.Icon,
		"helm/release_list":                    hlm_release_list.Icon,
		"helm/release_rollback":                hlm_release_rollback.Icon,
		"helm/release_status":                  hlm_release_status.Icon,
		"helm/release_test":                    hlm_release_test.Icon,
		"helm/release_uninstall":               hlm_release_uninstall.Icon,
		"helm/release_upgrade":                 hlm_release_upgrade.Icon,
	}
}

// destructiveActions permanently change cluster state and must be guarded.
func destructiveActions() map[string]bool {
	return map[string]bool{
		"helm/release_rollback":            true,
		"helm/release_uninstall":           true,
		"helm/release_upgrade":             true,
		"kubernetes/configmap_delete":      true,
		"kubernetes/cronjob_delete":        true,
		"kubernetes/daemonset_delete":      true,
		"kubernetes/deployment_delete":     true,
		"kubernetes/hpa_delete":            true,
		"kubernetes/ingress_delete":        true,
		"kubernetes/job_delete":            true,
		"kubernetes/namespace_delete":      true,
		"kubernetes/node_drain":            true,
		"kubernetes/pod_delete":            true,
		"kubernetes/pvc_delete":            true,
		"kubernetes/secret_delete":         true,
		"kubernetes/service_delete":        true,
		"kubernetes/serviceaccount_delete": true,
		"kubernetes/statefulset_delete":    true,

		// Vector Database ▸ pgvector: Delete Documents destroys stored documents,
		// so it carries the same confirm_destructive guard. Its inputs are checked
		// by the pgvector node's own drift test — the tables in this file only
		// cover infrastructure/*.
		"vectordatabase/pgvector/document_delete": true,
	}
}

// editorBadges are the badge glyphs present in editor/app/components/icons/paths.ts.
// A composed icon "base+badge" whose badge is missing renders as a "?" in the palette.
var editorBadges = map[string]bool{
	"list": true, "plus": true, "trash": true, "eye": true, "pencil": true,
	"play": true, "ban": true, "check": true, "xmark": true, "rotate-right": true,
	"rotate-left": true, "file-lines": true, "clock": true, "arrow-up": true,
	"arrow-down": true, "pause": true, "magnifying-glass": true, "code": true,
	"arrows-up-down": true,

	// Added for AAP / AWX (awx_inputs_drift_test.go). This map is a snapshot of the
	// editor's glyph set, not of any one node's usage, so there is exactly one of it
	// and a sibling node extends it rather than starting a second copy. Each of these
	// was checked against editor/app/components/icons/paths.ts before being added —
	// putting a name here that the editor does not ship turns this test from a guard
	// into a rubber stamp.
	"circle-stop": true, "user": true, "link": true, "terminal": true,
}

// scopedToThisFile reports whether an action ID belongs to one of the two nodes
// whose invariants THIS file enforces.
//
// The tables above only hold kubernetes/* and helm/*, so today this is redundant
// — which is exactly why it is written down. It states, at the point of use, that
// the per-node assertions below (one shared credential block, two icon bases, one
// destructive set) are properties of those two nodes and not of infrastructure/ as
// a whole, so a sibling node cannot be bolted into these tables by accident. AAP /
// AWX has its own seven-field credential block, its own `ansible` icon base and
// its own destructive set; it is covered by awx_inputs_drift_test.go.
func scopedToThisFile(id string) bool {
	return strings.HasPrefix(id, "kubernetes/") || strings.HasPrefix(id, "helm/")
}

// TestAuthBlockDoesNotDrift asserts that every action's first eight inputs
// reproduce kubernetes.AuthInputs exactly — same names, types, labels,
// placeholders, and visible_when conditions, in the same order.
func TestAuthBlockDoesNotDrift(t *testing.T) {
	want := kubernetes.AuthInputs

	for id, inputs := range actionInputs() {
		if !scopedToThisFile(id) {
			continue
		}
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field auth block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: auth input %d drifted\n got: %+v\nwant: %+v", id, i, inputs[i], want[i])
			}
		}
	}
}

// TestNoResourceInputShadowsACredential guards the input-name collision that
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is first — so a resource field sharing a
// credential's name would silently read the credential instead.
func TestNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range kubernetes.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range actionInputs() {
		if !scopedToThisFile(id) {
			continue
		}
		for _, c := range inputs[len(kubernetes.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name", id, c.Name)
			}
		}
	}
}

// TestDestructiveActionsAreGuarded pins the confirm_destructive contract: the
// guard is the last input, required, and a boolean (so the editor renders a
// checkbox that can also be bound to a variable).
func TestDestructiveActionsAreGuarded(t *testing.T) {
	destructive := destructiveActions()

	for id, inputs := range actionInputs() {
		if !scopedToThisFile(id) {
			continue
		}
		last := inputs[len(inputs)-1]
		guarded := last.Name == "confirm_destructive"

		if destructive[id] && !guarded {
			t.Errorf("%s: destructive, but its last input is %q, not confirm_destructive", id, last.Name)
		}
		if !destructive[id] && guarded {
			t.Errorf("%s: carries confirm_destructive but is not listed as destructive", id)
		}
		if guarded {
			if !last.Required {
				t.Errorf("%s: confirm_destructive must be Required", id)
			}
			if last.Type != core.ConnectionTypeBoolean {
				t.Errorf("%s: confirm_destructive must be a boolean, got %q", id, last.Type)
			}
		}
	}
}

// TestIconsResolve keeps every action's Icon inside the glyph set the editor ships.
func TestIconsResolve(t *testing.T) {
	for id, icon := range actionIcons() {
		if !scopedToThisFile(id) {
			continue
		}
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "kubernetes" && base != "helm" {
			t.Errorf("%s: icon base %q is neither kubernetes nor helm", id, base)
		}
		if !editorBadges[badge] {
			t.Errorf("%s: icon badge %q is not in editor/app/components/icons/paths.ts — it would render as \"?\"", id, badge)
		}
	}
}

// TestStandardOutputsPresent pins the three outputs the flow engine reads: success
// drives the soft-failure path, error carries the message, and tool_result is what
// the AI tool loop shows the model.
func TestStandardOutputsPresent(t *testing.T) {
	for id, outputs := range actionOutputs() {
		have := map[string]bool{}
		for _, o := range outputs {
			have[o.Name] = true
		}
		for _, required := range []string{"success", "error", "tool_result"} {
			if !have[required] {
				t.Errorf("%s: missing the %q output", id, required)
			}
		}
	}
}

// coveredElsewhere is how a sibling node registers the actions IT enforces, so
// that TestEveryRegisteredActionIsCovered can see them.
//
// Every file in package infrastructure_test that carries its own drift tables
// must add its action IDs here from an init(), keyed on the sub-group path exactly
// as the tables above are ("awx/job_wait"). awx_inputs_drift_test.go does this.
//
// This is the ONE seam that lets the coverage test span multiple nodes without
// forcing them to share a credential block or an icon base. It is deliberately not
// a "skip anything under awx/" prefix rule: an action must be named by SOME table
// in this package or the test fails, which is the whole point.
var coveredElsewhere = map[string]bool{}

// TestEveryRegisteredActionIsCovered closes the loop on the tables above.
//
// The manifest generator discovers actions by scanning for an exported Execute,
// so an action cannot be "forgotten" at registration time — it lands in
// actions.Actions automatically. What CAN be forgotten is adding it to a drift
// table, and then every invariant above silently stops covering it.
//
// This asserts the two sets are equal in both directions: nothing registered under
// infrastructure/ is unchecked by any file in this package, and nothing checked
// has been deleted from the tree.
func TestEveryRegisteredActionIsCovered(t *testing.T) {
	covered := actionInputs()

	registered := map[string]bool{}
	for id := range actions.Actions {
		if strings.HasPrefix(id, "infrastructure/") {
			// The tables are keyed on the sub-group path, e.g. "kubernetes/pod_list".
			registered[strings.TrimPrefix(id, "infrastructure/")] = true
		}
	}

	for id := range registered {
		if _, ok := covered[id]; ok {
			continue
		}
		if coveredElsewhere[id] {
			continue // a sibling file in this package enforces this one
		}
		t.Errorf("infrastructure/%s is registered in actions.Actions but no drift table in this package covers it — add it to this file's tables, or to a sibling's (see coveredElsewhere)", id)
	}
	for id := range covered {
		if !registered[id] {
			t.Errorf("%s is covered by this file's tables but is not registered in actions.Actions — did the directory move?", id)
		}
	}
	for id := range coveredElsewhere {
		if !registered[id] {
			t.Errorf("%s is covered by a sibling drift table but is not registered in actions.Actions — did the directory move?", id)
		}
	}
}

// TestHelmVersionPlaceholderMatchesThePin keeps the operator-facing text honest.
//
// The Inputs literal must be a constant expression the manifest generator can
// AST-parse, so the placeholder cannot interpolate helm.DefaultVersion at the
// call sites that spell it out. This asserts the two never drift.
func TestHelmVersionPlaceholderMatchesThePin(t *testing.T) {
	for id, inputs := range actionInputs() {
		if !strings.HasPrefix(id, "helm/") {
			continue
		}
		for _, c := range inputs {
			if c.Name != "helm_version" {
				continue
			}
			if !strings.Contains(c.Placeholder, helm.DefaultVersion) {
				t.Errorf("%s: helm_version placeholder %q does not name the pinned version %s",
					id, c.Placeholder, helm.DefaultVersion)
			}
		}
	}
}
