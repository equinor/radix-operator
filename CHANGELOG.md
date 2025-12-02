# Changelog

All notable changes to this project will be documented in this file.

## [1.106.0](https://github.com/equinor/radix-operator/compare/v1.105.0..v1.106.0) - 2025-12-02

### 🚀 Features

- Extend CRD status with with reconcile state and error (#1518) - ([ff805ee](https://github.com/equinor/radix-operator/commit/ff805ee7fc78130a8fd7b80419d3679a03490a47)) by @nilsgstrabo in [#1518](https://github.com/equinor/radix-operator/pull/1518)


### 🐛 Bug Fixes

- Grant operator access to RadixApplication status - ([a721ec6](https://github.com/equinor/radix-operator/commit/a721ec6f489c7cf02dbb58561b185e64af4370c0)) by @nilsgstrabo in [#1543](https://github.com/equinor/radix-operator/pull/1543)


## [1.105.0](https://github.com/equinor/radix-operator/compare/v1.104.1..v1.105.0) - 2025-11-28

### 🚀 Features

- E2e testing with KIND (#1514) - ([6cc7b29](https://github.com/equinor/radix-operator/commit/6cc7b29f72ee40f8bb4e55e03dca0f474ec2705c)) by @Richard87 in [#1514](https://github.com/equinor/radix-operator/pull/1514)

- Validate RadixApplication in Webhook (#1474) - ([daee0ef](https://github.com/equinor/radix-operator/commit/daee0ef71d7efba8a3f512560d9f3da22a8d2d60)) by @Richard87 in [#1474](https://github.com/equinor/radix-operator/pull/1474)


### ⚙️ Miscellaneous Tasks

- Enhance e2e test setup with configurable parallelism (#1537) - ([9f012f5](https://github.com/equinor/radix-operator/commit/9f012f575272d5f7876b07e4e15e53a086399c31)) by @nilsgstrabo in [#1537](https://github.com/equinor/radix-operator/pull/1537)


## [1.104.1](https://github.com/equinor/radix-operator/compare/v1.104.0..v1.104.1) - 2025-11-17

### 🐛 Bug Fixes

- Bug resolve tags for a commit (#1522) - ([05402f9](https://github.com/equinor/radix-operator/commit/05402f96c1b602e5029345d82639ba49b8f184f1)) by @Richard87 in [#1522](https://github.com/equinor/radix-operator/pull/1522)


## [1.104.0](https://github.com/equinor/radix-operator/compare/v1.103.1..v1.104.0) - 2025-11-07

### 🚀 Features

- Return a more descriptive error message. (#1507) - ([e599811](https://github.com/equinor/radix-operator/commit/e5998112a2d5a2090904ddbdad0019c5d2663d9d)) by @jacobsolbergholm in [#1507](https://github.com/equinor/radix-operator/pull/1507)


### 🐛 Bug Fixes

- Remove environment from radixdeployjob (#1501) - ([26a58af](https://github.com/equinor/radix-operator/commit/26a58afa16f9ed45e01601ad46b8ccade2e2fe76)) by @Richard87 in [#1501](https://github.com/equinor/radix-operator/pull/1501)


## [1.103.1](https://github.com/equinor/radix-operator/compare/v1.103.0..v1.103.1) - 2025-11-06

### 🐛 Bug Fixes

- *(operator)* Record k8s warning event when reconcile fails (#1502) - ([4f46bda](https://github.com/equinor/radix-operator/commit/4f46bda01f54393b8988156535ea661a896b63fc)) by @nilsgstrabo in [#1502](https://github.com/equinor/radix-operator/pull/1502)

- *(refactor)* Change generic type for controller queue (#1495) - ([b089542](https://github.com/equinor/radix-operator/commit/b089542e4e89af3edde1b5cef0175e037932f408)) by @nilsgstrabo in [#1495](https://github.com/equinor/radix-operator/pull/1495)

- Refactor error messages to use %w for wrapping errors (#1504) - ([4887c8e](https://github.com/equinor/radix-operator/commit/4887c8e6398da0ea090e03953223caa33f0064b3)) by @nilsgstrabo in [#1504](https://github.com/equinor/radix-operator/pull/1504)


## [1.103.0](https://github.com/equinor/radix-operator/compare/v1.102.2..v1.103.0) - 2025-11-04

### 🚀 Features

- Introduce a securityContext section with a runAsUser property. (#1455) - ([f2f7539](https://github.com/equinor/radix-operator/commit/f2f7539cb3b37e2e0407ce714ec937fcecce9755)) by @jacobsolbergholm in [#1455](https://github.com/equinor/radix-operator/pull/1455)


### 🐛 Bug Fixes

- Remove deprecated environment variables and constants from deployment configuration (#1489) - ([0179877](https://github.com/equinor/radix-operator/commit/017987727ad3973a1893bb3b4f3ad6d7c2f42204)) by @nilsgstrabo in [#1489](https://github.com/equinor/radix-operator/pull/1489)

- Dont marshall empty fields (#1493) - ([c74161c](https://github.com/equinor/radix-operator/commit/c74161cbd7e387a21acfd8defd0a93dd6d294520)) by @Richard87 in [#1493](https://github.com/equinor/radix-operator/pull/1493)


## New Contributors ❤️

* @jacobsolbergholm made their first contribution in [#1455](https://github.com/equinor/radix-operator/pull/1455)
## [1.102.2](https://github.com/equinor/radix-operator/compare/v1.102.1..v1.102.2) - 2025-10-23

### 🐛 Bug Fixes

- Remove clusterActiveEgressIps handling from configuration and tests (#1481) - ([d5397e4](https://github.com/equinor/radix-operator/commit/d5397e4d1a79a81e574a4a4181392b0e519afd87)) by @nilsgstrabo in [#1481](https://github.com/equinor/radix-operator/pull/1481)


## [1.102.1](https://github.com/equinor/radix-operator/compare/v1.102.0..v1.102.1) - 2025-10-21

### 🐛 Bug Fixes

- Pipeline runner memory limit (hotfix) (#1476) - ([06f1b30](https://github.com/equinor/radix-operator/commit/06f1b30189d65784a087bbf7c12b6aed18ca9b14)) by @Richard87 in [#1476](https://github.com/equinor/radix-operator/pull/1476)


## [1.102.0](https://github.com/equinor/radix-operator/compare/v1.101.0..v1.102.0) - 2025-10-15

### 🚀 Features

- Integrate code from radix-velero-plugin repo - ([e724ebc](https://github.com/equinor/radix-operator/commit/e724ebc0783b50bdb7bf019062adfa4ffd4f5d28)) by @nilsgstrabo in [#1454](https://github.com/equinor/radix-operator/pull/1454)

- Integrate code from radix-job-scheduler - ([490054d](https://github.com/equinor/radix-operator/commit/490054d1fe5655121ae8fe83b0df640fe69a1161)) by @nilsgstrabo in [#1461](https://github.com/equinor/radix-operator/pull/1461)

- Remove restored status handling from various components (#1467) - ([c392160](https://github.com/equinor/radix-operator/commit/c3921602d388297a52fc02e9f2ceef1432991164)) by @nilsgstrabo in [#1467](https://github.com/equinor/radix-operator/pull/1467)


### 🐛 Bug Fixes

- Set zerolog default context logger to prevent silent nil pointer panic - ([2854226](https://github.com/equinor/radix-operator/commit/2854226ffa4186dec87e03ea29cc293e9ec3d87b)) by @nilsgstrabo in [#1458](https://github.com/equinor/radix-operator/pull/1458)

- Remove velero-plugin package - ([06c820e](https://github.com/equinor/radix-operator/commit/06c820eead22be3a2f587c471dee6e44527583a2)) by @nilsgstrabo in [#1465](https://github.com/equinor/radix-operator/pull/1465)


## [1.101.0](https://github.com/equinor/radix-operator/compare/v1.100.0..v1.101.0) - 2025-09-18

### 🚀 Features

- Allow multiple platform user groups - ([66814c4](https://github.com/equinor/radix-operator/commit/66814c4abe00d8507a6ca354338388258eac33de)) by @nilsgstrabo in [#1444](https://github.com/equinor/radix-operator/pull/1444)


### ⚙️ Miscellaneous Tasks

- Remove rr-test.yaml (#1442) - ([16307e5](https://github.com/equinor/radix-operator/commit/16307e54fc524be8966aafe8f0bc8eb7cb884acd)) by @nilsgstrabo in [#1442](https://github.com/equinor/radix-operator/pull/1442)

- Remove deprecated build-push workflow - ([2b2ed03](https://github.com/equinor/radix-operator/commit/2b2ed034cdaa5162c6d31f11608645298d206d89)) by @nilsgstrabo in [#1447](https://github.com/equinor/radix-operator/pull/1447)


## [1.100.0](https://github.com/equinor/radix-operator/compare/v1.88.6..v1.100.0) - 2025-09-11

### 🚀 Features

- *(ci)* Configure new release workflows - ([d47633d](https://github.com/equinor/radix-operator/commit/d47633d05dc8bb6774f23c1f6feae9dad42bf2e4)) by @nilsgstrabo in [#1435](https://github.com/equinor/radix-operator/pull/1435)


## New Contributors ❤️

* @github-actions[bot] made their first contribution in [#1438](https://github.com/equinor/radix-operator/pull/1438)
## [1.88.6](https://github.com/equinor/radix-operator/compare/v1.88.5..v1.88.6) - 2025-09-09

### 🐛 Bug Fixes

- *(chart)* Use ghcr.io for acr and buildkit builder images (#1433) - ([5597832](https://github.com/equinor/radix-operator/commit/5597832d30304912991cf7023bb3463afadf05b7)) by @nilsgstrabo in [#1433](https://github.com/equinor/radix-operator/pull/1433)


## [1.88.5](https://github.com/equinor/radix-operator/compare/v1.88.2..v1.88.5) - 2025-09-08

### 🚀 Features

- Manage all image tags in helm chart (#1419) - ([a7bd807](https://github.com/equinor/radix-operator/commit/a7bd807080534a7daf5906298e6ca7e20c40c148)) by @Richard87 in [#1419](https://github.com/equinor/radix-operator/pull/1419)


### 🐛 Bug Fixes

- Remove hack and sync radix-app-id to all apps (#1429) - ([2acea13](https://github.com/equinor/radix-operator/commit/2acea13a3f5b1f5617fe6848510c136816e15b46)) by @Richard87 in [#1429](https://github.com/equinor/radix-operator/pull/1429)


## [1.88.2](https://github.com/equinor/radix-operator/compare/v1.88.1..v1.88.2) - 2025-09-04

### 🐛 Bug Fixes

- Git group read access for acr build tasks (#1425) - ([e4eb87f](https://github.com/equinor/radix-operator/commit/e4eb87fdd70880253a19617ef47bc93cc7b7fcc0)) by @Richard87 in [#1425](https://github.com/equinor/radix-operator/pull/1425)


## [1.88.1](https://github.com/equinor/radix-operator/compare/v1.87.4..v1.88.1) - 2025-09-02

### 🚀 Features

- Remove unnessecary init containers (nslookup/chmod) (#1420) - ([f7a6952](https://github.com/equinor/radix-operator/commit/f7a6952b28590429261acdafaa258453bbea72d4)) by @Richard87 in [#1420](https://github.com/equinor/radix-operator/pull/1420)


### 🐛 Bug Fixes

- Rename radix-operator folder (#1418) - ([d235ea4](https://github.com/equinor/radix-operator/commit/d235ea49c979926ac3a152be770c17322c02fa6d)) by @Richard87 in [#1418](https://github.com/equinor/radix-operator/pull/1418)

- Correct umask (#1422) - ([5cc968a](https://github.com/equinor/radix-operator/commit/5cc968a9ec655e19ea7d0617594e7af19c52c69e)) by @Richard87 in [#1422](https://github.com/equinor/radix-operator/pull/1422)


### ⚙️ Miscellaneous Tasks

- Bump chart version (#1423) - ([c667fcc](https://github.com/equinor/radix-operator/commit/c667fccec1bd743bb391c0cd7c8c7be7e8a7adea)) by @Richard87 in [#1423](https://github.com/equinor/radix-operator/pull/1423)


## [1.87.4](https://github.com/equinor/radix-operator/compare/v1.87.1..v1.87.4) - 2025-08-19

### 🚀 Features

- Replace bitnami/redis image (#1410) - ([5f883d9](https://github.com/equinor/radix-operator/commit/5f883d929248f10e701c95871f7deb94a50b15d8)) by @Richard87 in [#1410](https://github.com/equinor/radix-operator/pull/1410)

- Add option to disable request buffering (#1412) - ([1b7b92d](https://github.com/equinor/radix-operator/commit/1b7b92db5297c2b5e68768deedb728305be24cac)) by @Richard87 in [#1412](https://github.com/equinor/radix-operator/pull/1412)


### 🐛 Bug Fixes

- Missing ServeMonitors for Radix jobs (#1407) - ([8b874ff](https://github.com/equinor/radix-operator/commit/8b874ffc00bb0948af19dcbb60e4c013ac5bdd9e)) by @satr in [#1407](https://github.com/equinor/radix-operator/pull/1407)


## [1.87.1](https://github.com/equinor/radix-operator/compare/v1.87.0..v1.87.1) - 2025-07-30

### 🚀 Features

- Radix Deployment activated alert - ([d2bec21](https://github.com/equinor/radix-operator/commit/d2bec212ddf35ca4079d0f9203df7a340962bac8)) by @satr in [#1404](https://github.com/equinor/radix-operator/pull/1404)


## [1.87.0](https://github.com/equinor/radix-operator/compare/v1.86.1..v1.87.0) - 2025-07-18

### 🚀 Features

- Supported command and args in Radix jobs - ([0154bdc](https://github.com/equinor/radix-operator/commit/0154bdc374ff774a0bee0f9a36eca61c546b184d)) by @satr in [#1400](https://github.com/equinor/radix-operator/pull/1400)


## [1.86.1](https://github.com/equinor/radix-operator/compare/v1.86.0..v1.86.1) - 2025-07-17

### 🐛 Bug Fixes

- Make authentication in servicebus trigger optional (#1399) - ([fd00937](https://github.com/equinor/radix-operator/commit/fd00937efe32fe9a1be037c8718d4e9f74d8a144)) by @satr in [#1399](https://github.com/equinor/radix-operator/pull/1399)


## [1.86.0](https://github.com/equinor/radix-operator/compare/v1.85.1..v1.86.0) - 2025-07-15

### 🚀 Features

- Extended Radix scheduled jobs with image and variables - ([346cecf](https://github.com/equinor/radix-operator/commit/346cecf65a7a6fa4970e3ddff59d2b05588abf75)) by @satr in [#1397](https://github.com/equinor/radix-operator/pull/1397)


## [1.85.1](https://github.com/equinor/radix-operator/compare/v1.85.0..v1.85.1) - 2025-07-08

### 🐛 Bug Fixes

- Define pull image secrets for webhook and operator (#1394) - ([b5833cf](https://github.com/equinor/radix-operator/commit/b5833cf3ad6cac264fade830bd456b373538eb9f)) by @Richard87 in [#1394](https://github.com/equinor/radix-operator/pull/1394)


## [1.85.0](https://github.com/equinor/radix-operator/compare/v1.84.0..v1.85.0) - 2025-07-07

### 🚀 Features

- Add Admission webhook (#1385) - ([26d9cba](https://github.com/equinor/radix-operator/commit/26d9cbaf874b180f76f18b032a3c6b4d1aa8e8ca)) by @Richard87 in [#1385](https://github.com/equinor/radix-operator/pull/1385)


## [1.82.0](https://github.com/equinor/radix-operator/compare/v1.81.2..v1.82.0) - 2025-06-17

### 🚀 Features

- Add Radix AppID to components, jobs, batch jobs (#1379) - ([d94291f](https://github.com/equinor/radix-operator/commit/d94291ff3267ba9aa1e77da7f2b75b90f90187d1)) by @Richard87 in [#1379](https://github.com/equinor/radix-operator/pull/1379)


### 🐛 Bug Fixes

- Use correct radixjob name in label (#1375) - ([5c804b0](https://github.com/equinor/radix-operator/commit/5c804b0f99e9c4e0d8e563b7eeb9fd8911d5cd86)) by @Richard87 in [#1375](https://github.com/equinor/radix-operator/pull/1375)

- Use go-git checkout with force: true to discard local changes (#1376) - ([ebee1ad](https://github.com/equinor/radix-operator/commit/ebee1ad2391de914594c9f5805bd8fb56fae8783)) by @nilsgstrabo in [#1376](https://github.com/equinor/radix-operator/pull/1376)

- Check that before commit exists before diff with current commit - ([c05ee21](https://github.com/equinor/radix-operator/commit/c05ee218bce2a374220687c1ee5672d4557203e8)) by @nilsgstrabo

- Verify "before commit" exist when determining changed component in build-deploy pipelinne - ([4ef64e8](https://github.com/equinor/radix-operator/commit/4ef64e8cf9b3c51466efb9faee49ab0e29a2c41e)) by @nilsgstrabo in [#1378](https://github.com/equinor/radix-operator/pull/1378)


## [1.81.1](https://github.com/equinor/radix-operator/compare/v1.81.0..v1.81.1) - 2025-06-02

### 🐛 Bug Fixes

- Pipeline runner must checkout the specified commit instead of branch when cloning (#1370) - ([c883f3f](https://github.com/equinor/radix-operator/commit/c883f3f1761c19bbedf810cf81b26ac1c317ad1b)) by @nilsgstrabo in [#1370](https://github.com/equinor/radix-operator/pull/1370)


## [1.81.0](https://github.com/equinor/radix-operator/compare/v1.80.1..v1.81.0) - 2025-05-26

### 🐛 Bug Fixes

- Unable to promote RD when imageTagName is missing in target environment (#1363) - ([58e0dde](https://github.com/equinor/radix-operator/commit/58e0dde91f4d7d5a949f32fbd6df8ccd839e6732)) by @nilsgstrabo in [#1363](https://github.com/equinor/radix-operator/pull/1363)


## [1.80.1](https://github.com/equinor/radix-operator/compare/v1.80.0..v1.80.1) - 2025-05-20

### 🐛 Bug Fixes

- Remove unused image tags from RadixJob spec (#1358) - ([1f383a8](https://github.com/equinor/radix-operator/commit/1f383a89b395fa8c9c267656822e0b9c021e89f4)) by @Richard87 in [#1358](https://github.com/equinor/radix-operator/pull/1358)


## [1.79.1](https://github.com/equinor/radix-operator/compare/v1.79.0..v1.79.1) - 2025-05-09

### 🐛 Bug Fixes

- Unable to promote RD when imageTagName is missing in target environment (#1353) - ([c49b583](https://github.com/equinor/radix-operator/commit/c49b5835fb805300dfc5ca5e10173ce9c3814d5f)) by @nilsgstrabo in [#1353](https://github.com/equinor/radix-operator/pull/1353)


## [1.79.0](https://github.com/equinor/radix-operator/compare/v1.78.4..v1.79.0) - 2025-05-08

### 🐛 Bug Fixes

- Set memory and cpu requirements for pipeline-runner (#1346) - ([3462ab2](https://github.com/equinor/radix-operator/commit/3462ab235dff6c7c3740c78edb2c4acec0733b45)) by @Richard87 in [#1346](https://github.com/equinor/radix-operator/pull/1346)

- Remove extra indentation in chart template causing install to fail (#1356) - ([fbb099e](https://github.com/equinor/radix-operator/commit/fbb099e7ca8a5a7049d09c07a7e2b31084e1a743)) by @nilsgstrabo in [#1356](https://github.com/equinor/radix-operator/pull/1356)


## [1.78.4](https://github.com/equinor/radix-operator/compare/v1.78.1..v1.78.4) - 2025-05-05

### 🚀 Features

- Git clone without blobs (#1343) - ([f57c63a](https://github.com/equinor/radix-operator/commit/f57c63a1090700c8d88584d1685c416326a92917)) by @Richard87 in [#1343](https://github.com/equinor/radix-operator/pull/1343)


### 🐛 Bug Fixes

- Create a full clone (reverts sparse clone) (#1344) - ([098a354](https://github.com/equinor/radix-operator/commit/098a354522463e00fe13f1310e3607bdfe2c7bde)) by @Richard87 in [#1344](https://github.com/equinor/radix-operator/pull/1344)


### ⚙️ Miscellaneous Tasks

- Replace deprecated kubeutil.ApplySecret (#1342) - ([858e31c](https://github.com/equinor/radix-operator/commit/858e31cbdf4604612789d3cf324985d476b7fdde)) by @nilsgstrabo in [#1342](https://github.com/equinor/radix-operator/pull/1342)


## [1.77.0](https://github.com/equinor/radix-operator/compare/v1.76.2..v1.77.0) - 2025-04-14

### 🚀 Features

- Add field proxy buffer size to networkingresspublic in radixapplication (#1330) - ([6b073ac](https://github.com/equinor/radix-operator/commit/6b073ac75fd27faadf1e234a1ca2672024247be7)) by @nilsgstrabo in [#1330](https://github.com/equinor/radix-operator/pull/1330)


### ⚙️ Miscellaneous Tasks

- Update to go 1.24 and bump dependencies (#1328) - ([1928d6a](https://github.com/equinor/radix-operator/commit/1928d6afa01a34248ce43e076e9c6a9d892ed6fe)) by @nilsgstrabo in [#1328](https://github.com/equinor/radix-operator/pull/1328)


## [1.72.4](https://github.com/equinor/radix-operator/compare/v1.72.2..v1.72.4) - 2025-02-24

### 💼 Other

- Set max memory cache for streaming (#1297) - ([5296fec](https://github.com/equinor/radix-operator/commit/5296fecd1dfa8b8acd0cb01c7f85050bc148d2bf)) by @nilsgstrabo in [#1297](https://github.com/equinor/radix-operator/pull/1297)


## [1.68.0](https://github.com/equinor/radix-operator/compare/v1.67.0..v1.68.0) - 2024-12-11

### 🚀 Features

- Add Healtchchecks to RadixConfig - ([aa123c4](https://github.com/equinor/radix-operator/commit/aa123c413e98c02081fde905ea87a9ad4afebf42)) by @Richard87


## [1.66.3](https://github.com/equinor/radix-operator/compare/v1.66.0..v1.66.3) - 2024-11-22

### 🚀 Features

- Add radixdeployment crd (#1221) - ([35871e6](https://github.com/equinor/radix-operator/commit/35871e6a4334ac8d820c9374adba0bcc86549898)) by @Richard87 in [#1221](https://github.com/equinor/radix-operator/pull/1221)

- Add 503 annotation to all ingresses to capture 503 errors and show nice page (#1224) - ([4b44992](https://github.com/equinor/radix-operator/commit/4b4499247d1b3df3dab30f1c58b3dfa7797f3fda)) by @Richard87 in [#1224](https://github.com/equinor/radix-operator/pull/1224)


## [1.34.1](https://github.com/equinor/radix-operator/compare/v1.34.0..v1.34.1) - 2023-03-13

### 💼 Other

- Add empty string enum for non-pointer strings - ([b2874a6](https://github.com/equinor/radix-operator/commit/b2874a6f584dfc7e8c7bf8753264ca4de3387190)) by @nilsgstrabo


## [1.5.21](https://github.com/equinor/radix-operator/compare/v1.5.20..v1.5.21) - 2020-09-02

### 💼 Other

- Fix limitrange defined for app namespaces (#475) - ([b987e2a](https://github.com/equinor/radix-operator/commit/b987e2abcf10426c04793224411a8dd44aa21672)) by @keaaa in [#475](https://github.com/equinor/radix-operator/pull/475)


## [1.5.20](https://github.com/equinor/radix-operator/compare/v1.5.18..v1.5.20) - 2020-08-27

### 💼 Other

- Default resources reported when empty - ([e095d8d](https://github.com/equinor/radix-operator/commit/e095d8d2dd01e05555ee7b743a213be96fd0087d)) by @keaaa in [#468](https://github.com/equinor/radix-operator/pull/468)

- Update limitrange (#471) - ([e6f79b8](https://github.com/equinor/radix-operator/commit/e6f79b87c826cf7bfe2f9d0df5889b50fbe4b4a7)) by @keaaa in [#471](https://github.com/equinor/radix-operator/pull/471)


## [1.5.4](https://github.com/equinor/radix-operator/compare/v1.5.3..v1.5.4) - 2020-03-26

### 💼 Other

- Include apply radixconfig step in pipeline (#416) - ([4afd14b](https://github.com/equinor/radix-operator/commit/4afd14b86e501d17d8caba268d5aace0d7fd2334)) by @keaaa in [#416](https://github.com/equinor/radix-operator/pull/416)


## [1.4.4](https://github.com/equinor/radix-operator/compare/v1.4.3..v1.4.4) - 2020-03-03

### 💼 Other

- Changed creator to triggeredby (#403) - ([317b3a3](https://github.com/equinor/radix-operator/commit/317b3a3b8a1ef246893dc044304a7e37586fa55a)) by @keaaa in [#403](https://github.com/equinor/radix-operator/pull/403)


## [1.3.0](https://github.com/equinor/radix-operator/compare/v1.0.3..v1.3.0) - 2019-11-19

### 💼 Other

- Update rbac rules (#351) - ([e9a4d11](https://github.com/equinor/radix-operator/commit/e9a4d112c010355cc41d324b41181ed166eeb74d)) by @keaaa in [#351](https://github.com/equinor/radix-operator/pull/351)

- Fix order of private image hub transactions (#353) - ([c2194e6](https://github.com/equinor/radix-operator/commit/c2194e6b4dda8987fd7e2e9ee1ed0425e76101df)) by @keaaa in [#353](https://github.com/equinor/radix-operator/pull/353)


## [1.0.3](https://github.com/equinor/radix-operator/compare/v1.0.0..v1.0.3) - 2019-11-14

### 💼 Other

- Support private image hub - ([f2b6420](https://github.com/equinor/radix-operator/commit/f2b642050b2c643423641cd97abb5fc95a698b57)) by @keaaa in [#347](https://github.com/equinor/radix-operator/pull/347)


## [1.0.0](https://github.com/equinor/radix-operator/compare/v/1.2.0..v1.0.0) - 2019-10-28

### 💼 Other

- Update chart - ([abf42ef](https://github.com/equinor/radix-operator/commit/abf42ef66bf5b8c746ca4962930e59561d2b01b0)) by @StianOvrevage

- Ra validation on pipeline startup - ([d70b339](https://github.com/equinor/radix-operator/commit/d70b33935d8235e1efc26fe7604bb6e65758f8c3)) by @kjellerik

- Rename component secret name - ([990a3ad](https://github.com/equinor/radix-operator/commit/990a3ad6c83f89c1370d45ee6bd9a41e470de23f)) by @kjellerik

- Rename component secret name - ([2551db4](https://github.com/equinor/radix-operator/commit/2551db46d885e2c3591514a098687ba438e22cc7)) by @kjellerik

- Include dnsappalias to radixconfig - ([bec0b19](https://github.com/equinor/radix-operator/commit/bec0b199edab0ae9c70e45f5a72f3cd8215c301a)) by @kjellerik

- App url alias support - ([5256df3](https://github.com/equinor/radix-operator/commit/5256df3f15c662d54db45ed873826436dd6e2870)) by @kjellerik

- Adding spec for yaml serializing - ([91edd2b](https://github.com/equinor/radix-operator/commit/91edd2bf7c3b69798b9c3f2e28073d239e606a4f)) by @kjellerik

- Refactored deploy ingress handler - ([60ae902](https://github.com/equinor/radix-operator/commit/60ae9026c9a1a03023b096ec79ead9a8f2c78d6b)) by @kjellerik

- Url app alias moved to env variable - ([3dc3cde](https://github.com/equinor/radix-operator/commit/3dc3cdeccd282721dc43ebaf89254c31ac85e72b)) by @kjellerik

- Incl env variable in helm chart - ([9e6aedc](https://github.com/equinor/radix-operator/commit/9e6aedc8bce742d42a5d106b681e996b810e4e92)) by @kjellerik

- Fix incorrect prometheusName - ([55c5a8e](https://github.com/equinor/radix-operator/commit/55c5a8e7f893bcb243206443ac60999f505e9e31)) by @StianOvrevage

- Stricter validation of req resource values - ([192efd0](https://github.com/equinor/radix-operator/commit/192efd089d1dc785b63a9213a02cb9ceb206a234)) by @kjellerik

- Documentation updated - ([c2b5358](https://github.com/equinor/radix-operator/commit/c2b53580d7d03735bea0d13dbf973859baa72842)) by @kjellerik

- Change kaniko shapshotMode - ([9e6e866](https://github.com/equinor/radix-operator/commit/9e6e866ba4016bc1bb532b3a6b079b7dfbe4c02a)) by @kjellerik

- Adds timestamp to build job name - ([83e4bd3](https://github.com/equinor/radix-operator/commit/83e4bd3234ff35c966e892c92fcdf12d9a8e8ed8)) by @kjellerik

- Pod security for apps enforced by operator - ([e2900e6](https://github.com/equinor/radix-operator/commit/e2900e625d6b64ce4061dd767a16aa5ce473e605)) by @kjellerik

- Fix typos - ([ecc6ada](https://github.com/equinor/radix-operator/commit/ecc6ada41346ed06cf90a382276ab8ae278dd422)) by @kjellerik

- Disable runAsNonRoot and runAsUser, ruined webconsole - ([d7306dd](https://github.com/equinor/radix-operator/commit/d7306dd7342fe596103ce41065d77c26370af047)) by @kjellerik

- Clone app wildcard (#157) - ([ed7c39f](https://github.com/equinor/radix-operator/commit/ed7c39f3ea8076e72d2b24ce4ed3c4f6ec21c680)) by @ingeknudsen in [#157](https://github.com/equinor/radix-operator/pull/157)

- Force ssl redirect on ingresses - ([d4c220f](https://github.com/equinor/radix-operator/commit/d4c220f9f180047e25a8f7035a16ee406ea8d6c6)) by @kjellerik

- Fix app alias certificate - ([282a4ed](https://github.com/equinor/radix-operator/commit/282a4ed9618e8b2aadd0637aaa95761237a0ba8a)) by @kjellerik

- Network policy appied on deploy - ([25f7584](https://github.com/equinor/radix-operator/commit/25f758494dadd75142dadf65cb8fdfde06294b7e)) by @keaaa in [#206](https://github.com/equinor/radix-operator/pull/206)

- Build and build-deploy pipeline - ([ba1040b](https://github.com/equinor/radix-operator/commit/ba1040b45a95d7cc779ac37adddf2fde8e41d0d6)) by @keaaa in [#218](https://github.com/equinor/radix-operator/pull/218)

- Deny users creating k8s jobs - ([39f7d4b](https://github.com/equinor/radix-operator/commit/39f7d4b524d94ada51267c98f4962abad475f0ff)) by @keaaa in [#222](https://github.com/equinor/radix-operator/pull/222)

- Pipeline watches instead of pulling build status - ([f876b1b](https://github.com/equinor/radix-operator/commit/f876b1b377dcbaec5e0790140e58c9019d511944)) by @keaaa in [#228](https://github.com/equinor/radix-operator/pull/228)

- Measure requests and latency towards k8s api - ([731c5a3](https://github.com/equinor/radix-operator/commit/731c5a3d906f724994fdcd4891c6239d0658c724)) by @keaaa in [#229](https://github.com/equinor/radix-operator/pull/229)

- Fixed issue when config was not set - ([3372aa4](https://github.com/equinor/radix-operator/commit/3372aa46dab46374a6ea16d1397aca5751a30c57)) by @kjellerik

- Remove unused clusterroles - ([4126557](https://github.com/equinor/radix-operator/commit/412655752fb1222be6a901f885d68c89a565ff1e)) by @keaaa in [#248](https://github.com/equinor/radix-operator/pull/248)

- Radix operator with own role - ([6a5500a](https://github.com/equinor/radix-operator/commit/6a5500a8b4d770ba848f0e58fb8f8e65bbdbacb3)) by @keaaa in [#256](https://github.com/equinor/radix-operator/pull/256)

- Expose active cluster url as env var (#266) - ([d66ff2e](https://github.com/equinor/radix-operator/commit/d66ff2e3bb26254da61b3c88f3e67f70d97a52c5)) by @keaaa in [#266](https://github.com/equinor/radix-operator/pull/266)

- Introduce status on radix deployment (#269) - ([338c698](https://github.com/equinor/radix-operator/commit/338c698111b198f6e3a22ea523cfd215b6735112)) by @keaaa in [#269](https://github.com/equinor/radix-operator/pull/269)

- Dns alias handled on promotion (#270) - ([460fe05](https://github.com/equinor/radix-operator/commit/460fe055ad1dee634716e6679ecb2bee7cc08182)) by @keaaa in [#270](https://github.com/equinor/radix-operator/pull/270)

- Deny users creating k8s jobs - ([4f3f580](https://github.com/equinor/radix-operator/commit/4f3f58089230daceab5d8b28e115e1179a63d9d2)) by @keaaa in [#223](https://github.com/equinor/radix-operator/pull/223)

- Remove unused clusterroles - ([9bcab06](https://github.com/equinor/radix-operator/commit/9bcab06783d3182fe3c9f22f3bd8a8ab0ae1c9dc)) by @keaaa in [#249](https://github.com/equinor/radix-operator/pull/249)

- Set resource request/limits in chart (#320) - ([211220f](https://github.com/equinor/radix-operator/commit/211220ff1d83a6a506f27921c38329b043fc8bc9)) by @keaaa in [#320](https://github.com/equinor/radix-operator/pull/320)

- Set resource request/limits in chart - ([5d2c6d9](https://github.com/equinor/radix-operator/commit/5d2c6d9a5c45df1b41c4203be1e01c175fd58664)) by @keaaa in [#322](https://github.com/equinor/radix-operator/pull/322)


## [/1.0.0] - 2018-07-02

## New Contributors ❤️

* @jonaspetersorensen made their first contribution in [#43](https://github.com/equinor/radix-operator/pull/43)
* @thezultimate made their first contribution
* @ made their first contribution in [#42](https://github.com/equinor/radix-operator/pull/42)
* @FrodeHus made their first contribution
* @nemzes made their first contribution
<!-- generated by git-cliff -->
