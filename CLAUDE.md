# VPA Graceful Drain Controller - 프로젝트 CLAUDE.md

## 프로젝트 개요

VPA(Vertical Pod Autoscaler) Pod Eviction 시 서비스 중단을 최소화하는 Kubernetes Controller

### 핵심 목표
- VPA Pod Eviction 시 서비스 중단 최소화
- Finalizer 기반 graceful drain 메커니즘 구현
- `vpa-managed: "true"` 어노테이션 기반 Pod 식별
- 애플리케이션 수정 없이 Controller 레벨에서 해결
- 설정 기반 유연한 동작 제어

## 기술 스택

### 언어 및 프레임워크
- **Go**: 1.24.4
- **Controller-runtime**: v0.21.0
- **Client-go**: v0.33.1
- **Kubernetes API**: v0.33.1

### 아키텍처
- **패턴**: Controller Pattern (Kubernetes Operator)
- **구조**: MVC 기반 분리 (Controller, Handler, Config)
- **배포**: 컨테이너화된 Kubernetes Deployment

## 프로젝트 구조

```
├── cmd/controller/          # 메인 엔트리포인트
├── pkg/
│   ├── controller/         # Pod Controller 및 설정 관리
│   ├── finalizer/          # Graceful Drain 로직
│   └── util/              # 공통 유틸리티
├── config/samples/         # Kubernetes 매니페스트
├── docs/                  # 프로젝트 문서
└── bin/                   # 빌드된 바이너리
```

## 개발 로드맵

### ✅ Phase 1: 기본 Controller 구조 (완료)
- [x] Go 프로젝트 초기화
- [x] Pod Controller 기본 구현
- [x] `vpa-managed: "true"` 어노테이션 기반 Pod 식별
- [x] Finalizer 관리 시스템
- [x] ConfigMap 기반 설정
- [x] 기본 Graceful Drain 로직
- [x] 배포 매니페스트 및 RBAC

### 🔄 Phase 2: 고급 Drain 메커니즘 (진행 예정)
#### 2.1 고급 연결 상태 감지 (1주차)
- [ ] 네트워크 연결 모니터링 (TCP/UDP 소켓 상태)
- [ ] HTTP/gRPC 연결 감지
- [ ] Service Mesh 통합 (Istio, Linkerd)

#### 2.2 워크로드별 특화 처리 (2주차)
- [ ] StatefulSet 지원 (순서 보장, PVC 고려)
- [ ] DaemonSet 처리 (노드별 특수 처리)
- [ ] Job/CronJob 처리 (작업 완료 대기)

#### 2.3 확장된 설정 시스템 (3주차)
- [ ] Annotation 기반 개별 설정
- [ ] 워크로드 타입별 기본값
- [ ] 동적 설정 리로드

### 📊 Phase 3: 모니터링 및 옵저버빌리티 (2주)
- [ ] Prometheus 메트릭 수집
- [ ] 구조화된 로깅 (JSON 포맷)
- [ ] OpenTelemetry 분산 추적
- [ ] Grafana 대시보드

### 🧪 Phase 4: 테스트 및 검증 (2-3주)
- [ ] 단위 테스트 (90% 코드 커버리지 목표)
- [ ] E2E 테스트 (Kind/k3s 기반)
- [ ] 성능 테스트 및 카오스 엔지니어링
- [ ] 문서화 완성

### 🚀 Phase 5: 운영 준비 및 릴리스 (1-2주)
- [ ] 보안 강화 및 취약점 스캔
- [ ] CI/CD 파이프라인 구축
- [ ] Helm 차트 및 OLM 지원

## 개발 명령어

### 기본 개발 워크플로우
```bash
# 의존성 정리
go mod tidy

# 코드 포맷팅
make fmt

# 정적 분석
make vet

# 테스트 실행
make test

# 빌드
make build

# 로컬 실행
make run
```

### Kubernetes 배포
```bash
# 설정 및 RBAC 적용
make install

# Controller 배포
make deploy

# 배포 해제
make undeploy
```

### Docker 관련
```bash
# 이미지 빌드
make docker-build

# 이미지 푸시
make docker-push
```

## 핵심 파일 위치

### 메인 로직
- **Controller**: `pkg/controller/pod_controller.go:25` - Pod 감시 및 관리
- **Drain Handler**: `pkg/finalizer/drain_handler.go:28` - Graceful drain 로직
- **설정 관리**: `pkg/controller/config.go:48` - ConfigMap 기반 설정

### 진입점
- **Main**: `cmd/controller/main.go:25` - 애플리케이션 시작점

### 배포 관련
- **RBAC**: `config/samples/rbac.yaml` - 권한 설정
- **Deployment**: `config/samples/deployment.yaml` - 배포 설정
- **ConfigMap**: `config/samples/configmap.yaml` - 설정 예시

## VPA 관리 Pod 설정

VPA Graceful Drain Controller가 관리할 Pod를 지정하는 방법:

### 1. 기본 방법 (권장)
Pod 또는 Deployment 템플릿에 `vpa-managed: "true"` 어노테이션 추가:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  annotations:
    vpa-managed: "true"  # VPA Graceful Drain Controller가 관리
spec:
  containers:
  - name: app
    image: nginx
```

### 2. Deployment에서 사용
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      annotations:
        vpa-managed: "true"  # 생성되는 모든 Pod에 적용
    spec:
      containers:
      - name: app
        image: nginx
```

### 3. 기존 VPA 어노테이션 (호환성)
기존 VPA 환경에서는 다음 어노테이션들도 자동 감지:
- `vpa-updater.client.k8s.io/last-updated`
- `vpa.k8s.io/resource-name`

## 주요 설정 옵션

### Controller 설정
```bash
--config-map-name=vpa-graceful-drain-config      # ConfigMap 이름
--config-map-namespace=kube-system                # ConfigMap 네임스페이스
--leader-elect=true                               # Leader Election 활성화
--health-probe-bind-address=:8081                 # 헬스체크 포트
```

### ConfigMap 설정 예시
```yaml
data:
  gracePeriodSeconds: "30"      # Grace period (기본: 30초)
  drainTimeoutSeconds: "300"    # Drain timeout (기본: 300초)
  namespaceSelector: |          # 대상 namespace 설정
    {
      "include": ["default", "production"],
      "exclude": ["kube-system", "kube-public"]
    }
```

## 트러블슈팅

### 일반적인 문제들
1. **Controller가 Pod를 감지하지 못함**
   - VPA 어노테이션 확인: `vpa-updater.client.k8s.io/last-updated`
   - RBAC 권한 확인: Pod get/list/watch 권한
   - 네임스페이스 셀렉터 설정 확인

2. **Finalizer가 제거되지 않음**
   - Controller 로그 확인: `kubectl logs -n kube-system deployment/vpa-graceful-drain-controller`
   - Pod 상태 확인: `kubectl describe pod <pod-name>`
   - 수동 Finalizer 제거: `kubectl patch pod <pod-name> --type json -p='[{"op": "remove", "path": "/metadata/finalizers/0"}]'`

3. **설정이 적용되지 않음**
   - ConfigMap 존재 확인: `kubectl get configmap -n kube-system vpa-graceful-drain-config`
   - Controller 재시작: `kubectl rollout restart deployment -n kube-system vpa-graceful-drain-controller`

### 로그 레벨 조정
```bash
# 디버그 로그 활성화
kubectl patch deployment -n kube-system vpa-graceful-drain-controller -p '{"spec":{"template":{"spec":{"containers":[{"name":"controller","args":["--zap-log-level=debug"]}]}}}}'
```

## 성공 지표

### 기술적 KPI
- **가용성**: 99.9% 이상 Controller 가동률
- **성능**: 평균 Drain 시간 30초 이내
- **안정성**: 메모리 누수 없음, CPU 사용률 안정
- **테스트**: 90% 이상 코드 커버리지

### 비즈니스 KPI
- **서비스 중단 최소화**: VPA로 인한 서비스 중단 시간 80% 이상 단축
- **운영 효율성**: 수동 개입 없이 자동 처리
- **확장성**: 1000+ Pod 규모 클러스터 지원

## 참고 문서

- **개발 계획서**: `docs/DEVELOPMENT_PLAN.md`
- **아키텍처 문서**: `docs/ARCHITECTURE.md` (Phase 4에서 작성 예정)
- **API 레퍼런스**: `docs/API_REFERENCE.md` (Phase 4에서 작성 예정)
- **운영 가이드**: `docs/OPERATIONS.md` (Phase 5에서 작성 예정)

## 연관 프로젝트

- **VPA_Recommender**: `/Users/cho/cho/task-master/VPA_Recommender`
  - 리소스 추천값 계산 담당
  - 본 프로젝트와 독립적으로 운영

## 기여 가이드

### 코드 스타일
- Go 표준 포맷팅 사용 (`go fmt`)
- 변수명: camelCase
- 상수명: UPPER_SNAKE_CASE
- 함수명: PascalCase (public), camelCase (private)

### 커밋 메시지
```
feat: Add graceful drain handler for StatefulSet
fix: Resolve finalizer removal issue
docs: Update API reference documentation
test: Add unit tests for config parser
```

### PR 체크리스트
- [ ] 테스트 통과 (`make test`)
- [ ] 린트 통과 (`make vet`)
- [ ] 문서 업데이트 (필요시)
- [ ] 변경 사항 기록 (CHANGELOG.md)

---

이 문서는 프로젝트 진행에 따라 지속적으로 업데이트됩니다.
마지막 업데이트: Phase 1 완료 시점