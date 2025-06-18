# VPA Graceful Drain Controller - Project Context

## 프로젝트 기원
이 프로젝트는 VPA_Recommender 프로젝트 분석 중 도출된 요구사항을 기반으로 합니다.

## 핵심 요구사항
- VPA Pod Eviction 시 서비스 중단 최소화
- Finalizer 기반 graceful drain 메커니즘
- 애플리케이션 수정 없이 Controller 레벨에서 해결

## 기술 결정사항
- **언어**: Go 1.24.4
- **프레임워크**: controller-runtime v0.16.0, client-go v0.28.0
- **프로메테우스**: 사용하지 않음 (요구사항에 따라)
- **아키텍처**: 별도 프로젝트로 분리 (VPA_Recommender와 독립)

## 개발 로드맵
1. **Phase 1 (1주)**: 기본 Controller 구조
2. **Phase 2 (2-3주)**: Finalizer 기반 drain 메커니즘
3. **Phase 3 (4주)**: 설정 시스템 및 최적화
4. **Phase 4 (5주)**: 테스트 및 문서화

## 주요 피드백 반영사항
- deletionTimestamp 감지를 우선으로 시작
- ConfigMap에 namespaceSelector 지원
- Security Context 강화
- 설정 검증 로직 포함
- CRD는 Phase 2로 이후 검토

## 다음 단계
1. 새 디렉터리에서 프로젝트 초기화
2. Phase 1 개발 시작
3. go mod init 및 기본 구조 생성

## 연관 프로젝트
- **VPA_Recommender**: 리소스 추천값 계산
- **VPA Graceful Drain Controller**: 추천값의 안전한 적용

## 참고 링크
- Original PRD: [위에서 작성한 PRD 내용 참조]
- VPA_Recommender: /Users/cho/cho/task-master/VPA_Recommender