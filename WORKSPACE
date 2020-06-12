workspace(name = "goclassifieds")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/v0.19.5/rules_go-v0.19.5.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.19.5/rules_go-v0.19.5.tar.gz",
    ],
    sha256 = "513c12397db1bc9aa46dd62f02dd94b49a9b5d17444d49b5a04c5a89f3053c1c",
)

http_archive(
    name = "bazel_gazelle",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/0.18.2/bazel-gazelle-0.18.2.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/0.18.2/bazel-gazelle-0.18.2.tar.gz",
    ],
    sha256 = "7fc87f4170011201b1690326e8c16c5d802836e3a0d617d8f75c3af2b23180c4",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

go_repository(
    name = "com_github_aws_aws_lambda_go",
    importpath = "github.com/aws/aws-lambda-go",
    sum = "h1:Ogihmi8BnpmCNktKAGpNwSiILNNING1MiosnKUfU8m0=",
    version = "v1.17.0",
)

go_repository(
    name = "com_github_awslabs_aws_lambda_go_api_proxy",
    importpath = "github.com/awslabs/aws-lambda-go-api-proxy",
    sum = "h1:wbMkrj4dxSlW9BZ03mKip95YEodNcLGbZuP/ArG6+Ec=",
    version = "v0.6.0",
)

go_repository(
    name = "com_github_aymerick_raymond",
    importpath = "github.com/aymerick/raymond",
    sum = "h1:VEp3GpgdAnv9B2GFyTvqgcKvY+mfKMjPOA3SbKLtnU0=",
    version = "v2.0.2+incompatible",
)

go_repository(
    name = "com_github_bowery_prompt",
    importpath = "github.com/Bowery/prompt",
    sum = "h1:7tNlRGC3pUEPKS3DwgX5L0s+cBloaq/JBoi9ceN1MCM=",
    version = "v0.0.0-20190419144237-972d0ceb96f5",
)

go_repository(
    name = "com_github_burntsushi_toml",
    importpath = "github.com/BurntSushi/toml",
    sum = "h1:WXkYYl6Yr3qBf1K79EBnL4mak0OimBfB0XUf9Vl28OQ=",
    version = "v0.3.1",
)

go_repository(
    name = "com_github_cpuguy83_go_md2man_v2",
    importpath = "github.com/cpuguy83/go-md2man/v2",
    sum = "h1:U+s90UTSYgptZMwQh2aRr3LuazLJIa+Pg3Kc1ylSYVY=",
    version = "v2.0.0-20190314233015-f79a8a8ca69d",
)

go_repository(
    name = "com_github_davecgh_go_spew",
    importpath = "github.com/davecgh/go-spew",
    sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_dchest_safefile",
    importpath = "github.com/dchest/safefile",
    sum = "h1:3T8ZyTDp5QxTx3NU48JVb2u+75xc040fofcBaN+6jPA=",
    version = "v0.0.0-20151022103144-855e8d98f185",
)

go_repository(
    name = "com_github_eknkc_amber",
    importpath = "github.com/eknkc/amber",
    sum = "h1:clC1lXBpe2kTj2VHdaIu9ajZQe4kcEY9j0NsnDDBZ3o=",
    version = "v0.0.0-20171010120322-cdade1c07385",
)

go_repository(
    name = "com_github_elastic_go_elasticsearch_v7",
    importpath = "github.com/elastic/go-elasticsearch/v7",
    sum = "h1:QYOBmRtDpUK/EdlXUZcO9IsjIHVADqA05Lud940/ePA=",
    version = "v7.5.1-0.20200515132447-db2bcf51489c",
)

go_repository(
    name = "com_github_fatih_structs",
    importpath = "github.com/fatih/structs",
    sum = "h1:Q7juDM0QtcnhCpeyLGQKyg4TOIghuNXrkL32pHAUMxo=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_flosch_pongo2",
    importpath = "github.com/flosch/pongo2",
    sum = "h1:GY1+t5Dr9OKADM64SYnQjw/w99HMYvQ0A8/JoUkxVmc=",
    version = "v0.0.0-20190707114632-bbf5a6c351f4",
)

go_repository(
    name = "com_github_gin_contrib_sse",
    importpath = "github.com/gin-contrib/sse",
    sum = "h1:Y/yl/+YNO8GZSjAhjMsSuLt29uWRFHdHYUb5lYOV9qE=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_gin_gonic_gin",
    importpath = "github.com/gin-gonic/gin",
    sum = "h1:ahKqKTFpO5KTPHxWZjEdPScmYaGtLo8Y4DMHoEsnp14=",
    version = "v1.6.3",
)

go_repository(
    name = "com_github_go_check_check",
    importpath = "github.com/go-check/check",
    sum = "h1:0gkP6mzaMqkmpcJYCFOLkIBwI7xFExG03bbkOkCvUPI=",
    version = "v0.0.0-20180628173108-788fd7840127",
)

go_repository(
    name = "com_github_go_chi_chi",
    importpath = "github.com/go-chi/chi",
    sum = "h1:l4yNPeA/3kNJwE0uDBVXtFX8hfiHrlqkXBLPOrchWzk=",
    version = "v0.0.0-20180202194135-e223a795a06a",
)

go_repository(
    name = "com_github_go_playground_assert_v2",
    importpath = "github.com/go-playground/assert/v2",
    sum = "h1:MsBgLAaY856+nPRTKrp3/OZK38U/wa0CcBYNjji3q3A=",
    version = "v2.0.1",
)

go_repository(
    name = "com_github_go_playground_locales",
    importpath = "github.com/go-playground/locales",
    sum = "h1:HyWk6mgj5qFqCT5fjGBuRArbVDfE4hi8+e8ceBS/t7Q=",
    version = "v0.13.0",
)

go_repository(
    name = "com_github_go_playground_universal_translator",
    importpath = "github.com/go-playground/universal-translator",
    sum = "h1:icxd5fm+REJzpZx7ZfpaD876Lmtgy7VtROAbHHXk8no=",
    version = "v0.17.0",
)

go_repository(
    name = "com_github_go_playground_validator_v10",
    importpath = "github.com/go-playground/validator/v10",
    sum = "h1:nZU+7q+yJoFmwvNgv/LnPUkwPal62+b2xXj0AU1Es7o=",
    version = "v10.3.0",
)

go_repository(
    name = "com_github_golang_protobuf",
    importpath = "github.com/golang/protobuf",
    sum = "h1:+Z5KGCizgyZCbGh1KZqA0fcLLkwbsjIzS4aV2v7wJX0=",
    version = "v1.4.2",
)

go_repository(
    name = "com_github_google_go_cmp",
    importpath = "github.com/google/go-cmp",
    sum = "h1:xsAVV57WRhGj6kEIi8ReJzQlHHqcBYCElAvkovg3B/4=",
    version = "v0.4.0",
)

go_repository(
    name = "com_github_google_gofuzz",
    importpath = "github.com/google/gofuzz",
    sum = "h1:A8PeW59pxE9IoFRqBp37U+mSNaQoZ46F1f0f863XSXw=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_google_shlex",
    importpath = "github.com/google/shlex",
    sum = "h1:7+FW5aGwISbqUtkfmIpZJGRgNFg2ioYPvFaUxdqpDsg=",
    version = "v0.0.0-20181106134648-c34317bd91bf",
)

go_repository(
    name = "com_github_google_uuid",
    importpath = "github.com/google/uuid",
    sum = "h1:Gkbcsh/GbpXz7lPftLA3P6TYMwjCLYm83jiFQZF/3gY=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_gorilla_context",
    importpath = "github.com/gorilla/context",
    sum = "h1:AWwleXJkX/nhcU9bZSnZoi3h/qGYqQAGhq6zZe/aQW8=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_gorilla_mux",
    importpath = "github.com/gorilla/mux",
    sum = "h1:UsXWMy9j+GSCN/I1/Oyc4wGaeW2CDYqeqAkEvWPu+cs=",
    version = "v0.0.0-20180120075819-c0091a029979",
)

go_repository(
    name = "com_github_gorilla_schema",
    importpath = "github.com/gorilla/schema",
    sum = "h1:CamqUDOFUBqzrvxuz2vEwo8+SUdwsluFh7IlzJh30LY=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_iris_contrib_blackfriday",
    importpath = "github.com/iris-contrib/blackfriday",
    sum = "h1:o5sHQHHm0ToHUlAJSTjW9UWicjJSDDauOOQ2AHuIVp4=",
    version = "v2.0.0+incompatible",
)

go_repository(
    name = "com_github_iris_contrib_formbinder",
    importpath = "github.com/iris-contrib/formBinder",
    sum = "h1:jL+H+cCSEV8yzLwVbBI+tLRN/PpVatZtUZGK9ldi3bU=",
    version = "v5.0.0+incompatible",
)

go_repository(
    name = "com_github_iris_contrib_go_uuid",
    importpath = "github.com/iris-contrib/go.uuid",
    sum = "h1:XZubAYg61/JwnJNbZilGjf3b3pB80+OQg2qf6c8BfWE=",
    version = "v2.0.0+incompatible",
)

go_repository(
    name = "com_github_joker_hpp",
    importpath = "github.com/Joker/hpp",
    sum = "h1:PiDAizhfJbwZMISZ1Itx1ZTFeOFCml89Ofmz3V8rhoU=",
    version = "v0.0.0-20180418125244-6893e659854a",
)

go_repository(
    name = "com_github_joker_jade",
    importpath = "github.com/Joker/jade",
    sum = "h1:lOCEPvTAtWfLpSZYMOv/g44MGQFAolbKh2khHHGu0Kc=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_json_iterator_go",
    importpath = "github.com/json-iterator/go",
    sum = "h1:9yzud/Ht36ygwatGx56VwCZtlI/2AD15T1X2sjSuGns=",
    version = "v1.1.9",
)

go_repository(
    name = "com_github_juju_errors",
    importpath = "github.com/juju/errors",
    sum = "h1:rhqTjzJlm7EbkELJDKMTU7udov+Se0xZkWmugr6zGok=",
    version = "v0.0.0-20181118221551-089d3ea4e4d5",
)

go_repository(
    name = "com_github_juju_loggo",
    importpath = "github.com/juju/loggo",
    sum = "h1:MK144iBQF9hTSwBW/9eJm034bVoG30IshVm688T2hi8=",
    version = "v0.0.0-20180524022052-584905176618",
)

go_repository(
    name = "com_github_juju_testing",
    importpath = "github.com/juju/testing",
    sum = "h1:WQM1NildKThwdP7qWrNAFGzp4ijNLw8RlgENkaI4MJs=",
    version = "v0.0.0-20180920084828-472a3e8b2073",
)

go_repository(
    name = "com_github_kardianos_govendor",
    importpath = "github.com/kardianos/govendor",
    sum = "h1:WOH3FcVI9eOgnIZYg96iwUwrL4eOVx+aQ66oyX2R8Yc=",
    version = "v1.0.9",
)

go_repository(
    name = "com_github_kataras_golog",
    importpath = "github.com/kataras/golog",
    sum = "h1:Q/QxpyNBtfkhXE68tnEA4yyqm77eh/3YOjOw875VbBY=",
    version = "v0.0.0-20190624001437-99c81de45f40",
)

go_repository(
    name = "com_github_kataras_iris",
    importpath = "github.com/kataras/iris",
    sum = "h1:c2iRKvKLpTYMXKdVB8YP/+A67NtZFt9kFFy+ZwBhWD0=",
    version = "v11.1.1+incompatible",
)

go_repository(
    name = "com_github_kataras_pio",
    importpath = "github.com/kataras/pio",
    sum = "h1:V5Rs9ztEWdp58oayPq/ulmlqJJZeJP6pP79uP3qjcao=",
    version = "v0.0.0-20190103105442-ea782b38602d",
)

go_repository(
    name = "com_github_klauspost_compress",
    importpath = "github.com/klauspost/compress",
    sum = "h1:4UqAIzZ1Ns2epCTyJ1d2xMWvxtX+FNSCYWeOFogK9nc=",
    version = "v1.7.4",
)

go_repository(
    name = "com_github_klauspost_cpuid",
    importpath = "github.com/klauspost/cpuid",
    sum = "h1:vJi+O/nMdFt0vqm8NZBI6wzALWdA2X+egi0ogNyrC/w=",
    version = "v1.2.1",
)

go_repository(
    name = "com_github_kr_pretty",
    importpath = "github.com/kr/pretty",
    sum = "h1:L/CwN0zerZDmRFUapSPitk6f+Q3+0za1rQkzVuMiMFI=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_kr_pty",
    importpath = "github.com/kr/pty",
    sum = "h1:VkoXIwSboBpnk99O/KFauAEILuNHv5DVFKZMBN/gUgw=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_kr_text",
    importpath = "github.com/kr/text",
    sum = "h1:45sCR5RtlFHMR4UwH9sdQ5TC8v0qDQCHnXt+kaKSTVE=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_labstack_echo",
    importpath = "github.com/labstack/echo",
    sum = "h1:pGRcYk231ExFAyoAjAfD85kQzRJCRI8bbnE7CX5OEgg=",
    version = "v3.3.10+incompatible",
)

go_repository(
    name = "com_github_labstack_gommon",
    importpath = "github.com/labstack/gommon",
    sum = "h1:JvRqmeZcfrHC5u6uVleB4NxxNbzx6gpbJiQknDbKQu0=",
    version = "v0.2.8",
)

go_repository(
    name = "com_github_leodido_go_urn",
    importpath = "github.com/leodido/go-urn",
    sum = "h1:hpXL4XnriNwQ/ABnpepYM/1vCLWNDfUNts8dX3xTG6Y=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_mattn_go_colorable",
    importpath = "github.com/mattn/go-colorable",
    sum = "h1:G1f5SKeVxmagw/IyvzvtZE4Gybcc4Tr1tf7I8z0XgOg=",
    version = "v0.1.1",
)

go_repository(
    name = "com_github_mattn_go_isatty",
    importpath = "github.com/mattn/go-isatty",
    sum = "h1:wuysRhFDzyxgEmMf5xjvJ2M9dZoWAXNNr5LSBS7uHXY=",
    version = "v0.0.12",
)

go_repository(
    name = "com_github_mattn_goveralls",
    importpath = "github.com/mattn/goveralls",
    sum = "h1:7eJB6EqsPhRVxvwEXGnqdO2sJI0PTsrWoTMXEk9/OQc=",
    version = "v0.0.2",
)

go_repository(
    name = "com_github_microcosm_cc_bluemonday",
    importpath = "github.com/microcosm-cc/bluemonday",
    sum = "h1:5lPfLTTAvAbtS0VqT+94yOtFnGfUWYyx0+iToC3Os3s=",
    version = "v1.0.2",
)

go_repository(
    name = "com_github_modern_go_concurrent",
    importpath = "github.com/modern-go/concurrent",
    sum = "h1:TRLaZ9cD/w8PVh93nsPXa1VrQ6jlwL5oN8l14QlcNfg=",
    version = "v0.0.0-20180306012644-bacd9c7ef1dd",
)

go_repository(
    name = "com_github_modern_go_reflect2",
    importpath = "github.com/modern-go/reflect2",
    sum = "h1:9f412s+6RmYXLWZSEzVVgPGK7C2PphHj5RJrvfx9AWI=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_onsi_ginkgo",
    importpath = "github.com/onsi/ginkgo",
    sum = "h1:b2wg8HW/u55DT7Y/vamdEn/jdvtsGkxzl+0+iHa5YmE=",
    version = "v0.0.0-20180119174237-747514b53ddd",
)

go_repository(
    name = "com_github_onsi_gomega",
    importpath = "github.com/onsi/gomega",
    sum = "h1:yPHEatyQC4jN3vdfvqJXG7O9vfC6LhaAV1NEdYpP+h0=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_pkg_errors",
    importpath = "github.com/pkg/errors",
    sum = "h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=",
    version = "v0.9.1",
)

go_repository(
    name = "com_github_pmezard_go_difflib",
    importpath = "github.com/pmezard/go-difflib",
    sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_russross_blackfriday_v2",
    importpath = "github.com/russross/blackfriday/v2",
    sum = "h1:lPqVAte+HuHNfhJ/0LC98ESWRz8afy9tM/0RK8m9o+Q=",
    version = "v2.0.1",
)

go_repository(
    name = "com_github_ryanuber_columnize",
    importpath = "github.com/ryanuber/columnize",
    sum = "h1:j1Wcmh8OrK4Q7GXY+V7SVSY8nUWQxHW5TkBe7YUl+2s=",
    version = "v2.1.0+incompatible",
)

go_repository(
    name = "com_github_shopify_goreferrer",
    importpath = "github.com/Shopify/goreferrer",
    sum = "h1:WDC6ySpJzbxGWFh4aMxFFC28wwGp5pEuoTtvA4q/qQ4=",
    version = "v0.0.0-20181106222321-ec9c9a553398",
)

go_repository(
    name = "com_github_shurcool_sanitized_anchor_name",
    importpath = "github.com/shurcooL/sanitized_anchor_name",
    sum = "h1:PdmoCO6wvbs+7yrJyMORt4/BmY5IYyJwS/kOiWx8mHo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_stretchr_objx",
    importpath = "github.com/stretchr/objx",
    sum = "h1:4G4v2dO3VZwixGIRoQ5Lfboy6nUhCyYzaqnIAPPhYs4=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_stretchr_testify",
    importpath = "github.com/stretchr/testify",
    sum = "h1:nOGnQDM7FYENwehXlg/kFVnos3rEvtKTjRvOWSzb6H4=",
    version = "v1.5.1",
)

go_repository(
    name = "com_github_ugorji_go",
    importpath = "github.com/ugorji/go",
    sum = "h1:/68gy2h+1mWMrwZFeD1kQialdSzAb432dtpeJ42ovdo=",
    version = "v1.1.7",
)

go_repository(
    name = "com_github_ugorji_go_codec",
    importpath = "github.com/ugorji/go/codec",
    sum = "h1:2SvQaVZ1ouYrrKKwoSk2pzd4A9evlKJb9oTL+OaLUSs=",
    version = "v1.1.7",
)

go_repository(
    name = "com_github_urfave_cli_v2",
    importpath = "github.com/urfave/cli/v2",
    sum = "h1:Qt8FeAtxE/vfdrLmR3rxR6JRE0RoVmbXu8+6kZtYU4k=",
    version = "v2.1.1",
)

go_repository(
    name = "com_github_urfave_negroni",
    importpath = "github.com/urfave/negroni",
    sum = "h1:eg5xqGZGatsyRpVnFJkdeUWSFk46lDgkXLvOryv5ySg=",
    version = "v0.0.0-20180130044549-22c5532ea862",
)

go_repository(
    name = "com_github_valyala_bytebufferpool",
    importpath = "github.com/valyala/bytebufferpool",
    sum = "h1:GqA5TC/0021Y/b9FG4Oi9Mr3q7XYx6KllzawFIhcdPw=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_valyala_fasttemplate",
    importpath = "github.com/valyala/fasttemplate",
    sum = "h1:tY9CJiPnMXf1ERmG2EyK7gNUd+c6RKGD0IfU8WdUSz8=",
    version = "v1.0.1",
)

go_repository(
    name = "in_gopkg_check_v1",
    importpath = "gopkg.in/check.v1",
    sum = "h1:yhCVgyC4o1eVCa2tZl7eS0r+SDo693bJlVdllGtEeKM=",
    version = "v0.0.0-20161208181325-20d25e280405",
)

go_repository(
    name = "in_gopkg_go_playground_validator_v8",
    importpath = "gopkg.in/go-playground/validator.v8",
    sum = "h1:lFB4DoMU6B626w8ny76MV7VX6W2VHct2GVOI3xgiMrQ=",
    version = "v8.18.2",
)

go_repository(
    name = "in_gopkg_mgo_v2",
    importpath = "gopkg.in/mgo.v2",
    sum = "h1:xcEWjVhvbDy+nHP67nPDDpbYrY+ILlfndk4bRioVHaU=",
    version = "v2.0.0-20180705113604-9856a29383ce",
)

go_repository(
    name = "in_gopkg_yaml_v2",
    importpath = "gopkg.in/yaml.v2",
    sum = "h1:clyUAQHOM3G0M3f5vQj7LuJrETvjVot3Z5el9nffUtU=",
    version = "v2.3.0",
)

go_repository(
    name = "org_golang_google_protobuf",
    importpath = "google.golang.org/protobuf",
    sum = "h1:4MY060fB1DLGMB/7MBTLnwQUY6+F09GEiz6SsrNqyzM=",
    version = "v1.23.0",
)

go_repository(
    name = "org_golang_x_crypto",
    importpath = "golang.org/x/crypto",
    sum = "h1:ObdrDkeb4kJdCP557AjRjq69pTHfNouLtWZG7j9rPN8=",
    version = "v0.0.0-20191011191535-87dc89f01550",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:0mm1VjtFUOIlE1SbDlwjYaDxZVDP2S5ou6y0gSgXHu8=",
    version = "v0.0.0-20200226121028-0de0cce0169b",
)

go_repository(
    name = "org_golang_x_sync",
    importpath = "golang.org/x/sync",
    sum = "h1:WXEvlFVvvGxCJLG6REjsT03iWnKLEWinaScsxF2Vm2o=",
    version = "v0.0.0-20200317015054-43a5402ce75a",
)

go_repository(
    name = "org_golang_x_sys",
    importpath = "golang.org/x/sys",
    sum = "h1:YTzHMGlqJu67/uEo1lBv0n3wBXhXNeUbB1XfN2vmTm0=",
    version = "v0.0.0-20200515095857-1151b9dac4a9",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:tW2bmiBqwgJj/UpqtC8EpXEZVYOwU0yG4iWbprSVAcs=",
    version = "v0.3.2",
)

go_repository(
    name = "org_golang_x_tools",
    importpath = "golang.org/x/tools",
    sum = "h1:NXNmtp0ToD36cui5IqWy95LC4Y6vT/4y3RnPxlQPinU=",
    version = "v0.0.0-20200417140056-c07e33ef3290",
)

go_repository(
    name = "org_golang_x_xerrors",
    importpath = "golang.org/x/xerrors",
    sum = "h1:E7g+9GITq07hpfrRu66IVDexMakfv52eLZ2CXBWiKr4=",
    version = "v0.0.0-20191204190536-9bdfabe68543",
)

go_repository(
    name = "com_github_aws_aws_sdk_go",
    importpath = "github.com/aws/aws-sdk-go",
    sum = "h1:UeWMTRTL0XAKLR7vxDL4/u7KOtz/LtfJr+lXtxN4YEQ=",
    version = "v1.31.13",
)

go_repository(
    name = "com_github_go_sql_driver_mysql",
    importpath = "github.com/go-sql-driver/mysql",
    sum = "h1:ozyZYNQW3x3HtqT1jira07DN2PArx2v7/mN66gGcHOs=",
    version = "v1.5.0",
)

go_repository(
    name = "com_github_jmespath_go_jmespath",
    importpath = "github.com/jmespath/go-jmespath",
    sum = "h1:OS12ieG61fsCg5+qLJ+SsW9NicxNkg3b25OyT2yCeUc=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_dgrijalva_jwt_go",
    importpath = "github.com/dgrijalva/jwt-go",
    sum = "h1:7qlOGliEKZXTDg6OTjfoBKDXWrumCAMpl/TFQ4/5kLM=",
    version = "v3.2.0+incompatible",
)

go_repository(
    name = "com_github_mitchellh_mapstructure",
    importpath = "github.com/mitchellh/mapstructure",
    sum = "h1:iDwIio/3gk2QtLLEsqU5lInaMzos0hDTz8a6lazSFVw=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_tangzero_inflector",
    importpath = "github.com/tangzero/inflector",
    sum = "h1:933dvPwRUUOAl98hyeeXuzFix3HwDt5j+45lleu8oh0=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_bitly_go_hostpool",
    importpath = "github.com/bitly/go-hostpool",
    sum = "h1:mXoPYz/Ul5HYEDvkta6I8/rnYM5gSdSV2tJ6XbZuEtY=",
    version = "v0.0.0-20171023180738-a3a6125de932",
)

go_repository(
    name = "com_github_bmizerany_assert",
    importpath = "github.com/bmizerany/assert",
    sum = "h1:DDGfHa7BWjL4YnC6+E63dPcxHo2sUxDIu8g3QgEJdRY=",
    version = "v0.0.0-20160611221934-b7ed37b82869",
)

go_repository(
    name = "com_github_gocql_gocql",
    importpath = "github.com/gocql/gocql",
    sum = "h1:ehjJsmZzqcE3nGO1uKvEKOIflkdUyEjTDP7fA2m7f2c=",
    version = "v0.0.0-20200608162118-cb62e193e52b",
)

go_repository(
    name = "com_github_golang_snappy",
    importpath = "github.com/golang/snappy",
    sum = "h1:Qgr9rKW7uDUkrbSmQeiDsGa8SjGyCOGtuasMWwvp2P4=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_hailocab_go_hostpool",
    importpath = "github.com/hailocab/go-hostpool",
    sum = "h1:5upAirOpQc1Q53c0bnx2ufif5kANL7bfZWcc6VJWJd8=",
    version = "v0.0.0-20160125115350-e80d13ce29ed",
)

go_repository(
    name = "in_gopkg_inf_v0",
    importpath = "gopkg.in/inf.v0",
    sum = "h1:73M5CoZyi3ZLMOyDlQh031Cx6N9NDJ2Vvfl76EDAgDc=",
    version = "v0.9.1",
)

go_repository(
    name = "com_github_scylladb_go_reflectx",
    importpath = "github.com/scylladb/go-reflectx",
    sum = "h1:b917wZM7189pZdlND9PbIJ6NQxfDPfBvUaQ7cjj1iZQ=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_scylladb_gocqlx_v2",
    importpath = "github.com/scylladb/gocqlx/v2",
    sum = "h1:V6U8N0aHvPPZzD2PoDvQFmHKFokzXBJU7U/rDF33I4A=",
    version = "v2.0.3",
)

go_repository(
    name = "com_github_lestrrat_go_iter",
    importpath = "github.com/lestrrat-go/iter",
    sum = "h1:FvnrqecqX4zT0wOIbYK1gNgTm0677INEWiFY8UEYggY=",
    version = "v0.0.0-20200422075355-fc1769541911",
)

go_repository(
    name = "com_github_lestrrat_go_jwx",
    importpath = "github.com/lestrrat-go/jwx",
    sum = "h1:FsbZg/v979RikHWhSu/7BRHh2Z1Z8byPleURRb1Y0XI=",
    version = "v1.0.2",
)

go_repository(
    name = "com_github_lestrrat_go_pdebug",
    importpath = "github.com/lestrrat-go/pdebug",
    sum = "h1:aEZT3f1GGg5RIlHMAy4/4fe4ciOi3SCwYoaURphcB4k=",
    version = "v0.0.0-20200204225717-4d6bd78da58d",
)

go_repository(
    name = "com_github_yuin_goldmark",
    importpath = "github.com/yuin/goldmark",
    sum = "h1:nqDD4MMMQA0lmWq03Z2/myGPYLQoXtmi0rGVs95ntbo=",
    version = "v1.1.27",
)

go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
    sum = "h1:KU7oHjnv3XNWfa5COkzUifxZmxp1TyI7ImMXqFxLwvQ=",
    version = "v0.2.0",
)
