
resource "helm_release" "keda" {
  name             = "keda"
  repository       = "https://kedacore.github.io/charts"
  chart            = "keda"
  namespace        = "keda"
  create_namespace = true
  version          = "2.16.0"
}

resource "helm_release" "keda_http_add_on" {
  name             = "keda-add-ons-http"
  repository       = "https://kedacore.github.io/charts"
  chart            = "keda-add-ons-http"
  namespace        = "keda"
  create_namespace = true
  version          = "0.9.0"

  depends_on = [helm_release.keda]
}
