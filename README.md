# VPA Graceful Drain Controller

VPA(Vertical Pod Autoscaler) Pod Eviction 시 서비스 중단을 최소화하는 Kubernetes Controller입니다.

## 주요 기능

- VPA Pod Eviction 시 Finalizer 기반 graceful drain 메커니즘
- 애플리케이션 수정 없이 Controller 레벨에서 처리
- ConfigMap을 통한 설정 관리
- Namespace 별 선택적 적용

## 설치 및 실행

### 로컬 개발 환경

```bash
# 빌드
make build

# 실행
make run

# 테스트
make test
```

### Kubernetes 클러스터 배포

```bash
# 설정 및 RBAC 적용
make install

# Controller 배포
make deploy
```

## 설정

ConfigMap을 통해 Controller 동작을 설정할 수 있습니다:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vpa-graceful-drain-config
  namespace: kube-system
data:
  gracePeriodSeconds: "30"      # Grace period (기본: 30초)
  drainTimeoutSeconds: "300"    # Drain timeout (기본: 300초)
  namespaceSelector: |          # 대상 namespace 설정
    {
      "include": ["default", "production"],
      "exclude": ["kube-system", "kube-public"]
    }
```

## 개발 단계

- [x] **Phase 1**: 기본 Controller 구조
- [x] **Phase 2**: Finalizer 기반 drain 메커니즘
- [x] **Phase 3**: 설정 시스템 및 최적화
- [x] **Phase 4**: 테스트 및 문서화

## 아키텍처

```
Pod (with VPA annotation) → Controller → Finalizer → Graceful Drain → Pod Deletion
```

## 라이선스

Apache 2.0