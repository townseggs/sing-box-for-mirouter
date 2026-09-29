# HANDOFF：把上游某个 tag 的快照提交到本仓库

本仓库 `townseggs/sing-box-for-router` 只保存上游 `SagerNet/sing-box` 的 **tag 快照**，
**不包含上游历史**。每个 tag 对应一个没有父提交的孤立提交（root commit），
文件内容与上游该 tag 的 tree 完全一致。

## 一、身份与权限（每次都要确认）

- 本机有两套 SSH 身份，必须用对子：

| 地址写法 | 密钥 | GitHub 账号 | 对本仓库 |
| --- | --- | --- | --- |
| `git@github.com-townseggs:townseggs/sing-box-for-router.git` | `~/.ssh/id_ed25519_townseggs` | townseggs | 有读写权限 |
| `git@github.com:...`（默认） | `~/.ssh/id_rsa` | hovvsoon | 无权限（私有仓库会报 Repository not found） |

- 私钥权限必须是 `600`，否则 ssh 会拒绝加载（报 `UNPROTECTED PRIVATE KEY FILE`）：

  ```bash
  chmod 600 ~/.ssh/id_ed25519_townseggs
  ```

- 先验证身份，应输出 `Hi townseggs!`：

  ```bash
  ssh -T git@github.com-townseggs
  ```

- 提交身份：在 `~/townseggs` 目录下的仓库会自动使用 `townseggs <townseggs@gmail.com>`。
  如果换到别的目录操作，记得显式设置：

  ```bash
  git config user.name townseggs
  git config user.email townseggs@gmail.com
  ```

## 二、分支规则：main 是上游，`patch` 是我的改动

两条线，一条不放自己的东西，一条只放自己的东西：

- `main` = **上游分支**。内容永远等于某个上游 tag 的快照，保持干净，
  不放任何自己的改动。上游发新 tag，就把 `main` 整体换成那个 tag 的内容。
- `patch` = **改动分支**。所有定制改造都提交在这条上，从 `main` 分叉，
  名字固定、不带版本号，长期就这一条。

```
main:  858570c   ← v1.12.25 快照，纯净上游
           │
           └── patch   ← 我的改动都在这一条上

（上游出新版之后）

main:  xxxxxxx   ← v1.13.0 快照，纯净上游
           │
           └── patch   ← 重置到新 main，重新写定制
```

每次动手的顺序：

1. 让 `main` 对齐到你要基于的上游 tag（升版本时先更新 `main`）。
2. `patch` 分支落在该版本上：还没有 `patch` 时创建，

   ```bash
   git switch main
   git switch -c patch
   git push -u origin patch
   ```

   已经有 `patch` 时按下面的升级流程重置，不再新建分支。
3. 改动只提交在 `patch` 上，`main` 保持不动。
4. 上游升级时：`main` 前进到新 tag，再把 `patch` 重置到新 `main` 上重写定制。

**升级时怎么处理旧改动：** `main` 每次是孤立快照整体替换，各版本之间没有共同祖先，
所以 `patch` **不能 merge 新 `main`**。做法是把 `patch` 重置到新 `main`，
在该版本上**重新写一遍定制**：

```bash
# 0) 旧版本的改动先留档，否则重置后就不在分支上了
git tag 1.12.25-patch-archive patch
git push origin 1.12.25-patch-archive     # 想留到远端就推这个 tag

# 1) 先把 main 换成新 tag 的快照（见第三节）

# 2) patch 重置到新 main，重写定制
git switch patch
git fetch origin
git reset --hard origin/main
# 在新版基础上重新实现定制，然后提交
git push --force-with-lease origin patch   # patch 历史被重写，必须 force
```

即使是小版本升级（例如 `v1.12.25` → `v1.12.26`），也照样基于新版本重新写一遍，
不做 cherry-pick，也不用 format-patch 搬旧补丁：唯一的升级方式就是"换新 `main`、重写定制"。
好处是每次升级都等于按新版本重做一遍定制，不会积累"旧补丁看着还能打、实际语义已经和新版对不上"的隐患。

分支名不带版本号，所以版本信息由留档 tag 承载：对比不同版本上的定制用
`git diff 1.12.25-patch-archive patch`。

实践建议：把定制尽量集中到少数文件或一个目录（构建脚本、配置、自己新增的文件），
重写成本就低、升级也快；改到上游核心源码的话，每升一版都要重新对一次。

## 三、下次要加新 tag 快照（以 v1.13.0 为例）

```bash
TAG=v1.13.0
REPO=git@github.com-townseggs:townseggs/sing-box-for-router.git

# 1) 按 tag 浅克隆，只取该 tag 的快照，不要完整历史
git clone --branch "$TAG" --depth 1 https://github.com/SagerNet/sing-box.git "sing-box-$TAG"
cd "sing-box-$TAG"
SRC=$(git rev-parse HEAD)          # 记下上游提交，用于校验内容一致

# 2) 做成孤立快照提交（checkout --orphan 会保留文件并全部暂存，不会清空工作区）
git checkout --orphan main
git diff --cached "$SRC" --stat    # 应无输出；有输出说明文件与上游不一致，停下来查
git commit -q -m "sing-box $TAG tag snapshot"

# 3) 让 tag 指向这个快照提交
git tag -f "$TAG"

# 4) 指向自己的仓库并推送
git remote rename origin upstream
git remote add origin "$REPO"
git push --force-with-lease origin main
git push origin "refs/tags/$TAG"

# 5) 核对远端
git ls-remote origin
```

## 四、几个必须知道的坑

1. **必须用 force 推送 main。**
   每个快照提交都是孤立提交，和远端已有的 main 没有共同祖先，普通 `git push origin main`
   会被拒绝（non-fast-forward）。用 `--force-with-lease`，比 `--force` 安全：
   如果远端在你操作期间被别人改过，它会拒绝而不是覆盖。

2. **本文件（HANDOFF.md）会被下次快照覆盖掉。**
   它只存在于 `patch` 分支，`main` 保持纯上游快照、不含任何自己的文件。
   升级时新快照是全新的孤立提交，tree 直接取自上游，所以要在重置之后把它取回来，
   从上一版的留档 tag 拿最稳（`origin/patch` 一旦 force 推送就是新的了）：

   ```bash
   git fetch origin --tags
   git checkout 1.12.25-patch-archive -- HANDOFF.md
   git add HANDOFF.md      # 在 commit 之前执行
   ```

   要么把这份说明另存一份到仓库之外。

3. **同名 tag 的哈希与上游不同。**
   `v1.12.25` 在这里指向快照提交，而不是上游的 `73bfb99`。名字相同、内容相同、提交不同，
   以后对比时以「tree 相同」为准，不要拿 commit hash 和上游比。

4. **不要在快照之间做 merge。**
   两个 tag 的孤立提交没有共同祖先，merge 或 `git diff <tagA> <tagB>` 得不到有意义的上游变更；
   要比就比文件差异。想要「带历史的镜像」只能改用完整克隆，那时现有孤立历史会被替换掉。

5. **替代方案（如果想保留历次快照记录）：**
   不要在孤立分支上强行推 main，而是每次在现有 main 之上追加一个普通提交
   （写入新快照、删掉旧文件），这样是快进推送，不需要 force，`main` 的历史就是快照列表；
   代价是每个提交都很大，"当前是哪个 tag"要看最新提交。

## 五、本次已完成的记录

- tag：`v1.12.25`（上游提交 `73bfb99`，浅克隆 `--depth 1`）
- 快照提交：`a39e415`，消息 `sing-box v1.12.25 tag snapshot`
- tree：`33fc0de46910b2b2eaac6eff3e917c51bbb318a0`（与上游 v1.12.25 的 tree 一致）
- 内容：912 个文件，101734 行
- 远端：`refs/tags/v1.12.25` 指向 `a39e415`（纯快照）；`main` 在该快照之上追加了本文档等提交
- 分支：`main` 指向 `a39e415`（与 tag `v1.12.25` 同一个提交，纯上游快照，不含本文件）；
  `patch` 指向 `f5677b1`（快照 + 本文件），改动分支

## 六、速查（日常两条操作）

当前状态：本地 `main` 在 `a39e415`（纯快照，等同 tag `v1.12.25`），`patch` 在 `f5677b1`（含本文件）。

**1. 改定制逻辑 —— 只在 `patch` 上改，`main` 不碰**

```bash
git switch patch
# 改代码、提交
git push origin patch
```

**2. 升级上游快照（以 v1.13.0 为例）**

```bash
# 2.1 旧定制先留档，否则重置后就没了
git tag 1.12.25-patch-archive patch
git push origin 1.12.25-patch-archive

# 2.2 浅克隆新 tag → 做成孤立快照 → 更新 main（完整步骤和内容校验见第三节）
git clone --branch v1.13.0 --depth 1 https://github.com/SagerNet/sing-box.git sing-box-v1.13.0
cd sing-box-v1.13.0
git remote rename origin upstream
git remote add origin git@github.com-townseggs:townseggs/sing-box-for-router.git
git fetch origin main
git checkout --orphan main
git commit -m "sing-box v1.13.0 tag snapshot"   # main 只放纯快照，不带自己的文件
git tag -f v1.13.0
git push --force-with-lease origin main
git push origin refs/tags/v1.13.0

# 2.3 patch 重置到新 main，重新写定制
git switch patch
git fetch origin
git reset --hard origin/main
git checkout 1.12.25-patch-archive -- HANDOFF.md   # 本文件只在 patch 上，需取回
# 按新版本重新实现定制，提交
git push --force-with-lease origin patch
```

两条硬规则：

- `main`：普通快进推进用 `git push`；换 tag 做整体替换时才需要 `--force-with-lease`。
- `patch`：`git reset --hard` 之后历史被重写，推送必须 `--force-with-lease`。

## 七、发布（Release）

> **⚠️ 分工：推送和发布一律手动执行，工具不碰远端。**
>
> | 步骤 | 谁做 |
> | --- | --- |
> | 改代码、构建、脱敏、算校验和 | 工具可以本地做 |
> | `git commit` / `git push`（patch 分支提交） | **人工手动** |
> | 打 tag 并推送 | **人工手动** |
> | 在 fork 上创建 Release、上传产物 | **人工手动** |
>
> 原因：推送与发布是远端状态变更，且本仓库的 SSH 身份（`github.com-townseggs`）与凭据
> 由人掌握；工具只在本地产出可审阅的改动和产物，由人确认后再决定推不推、发不发。

### 7.1 产物清单

一次发布产出 **5 个产物**（每个只出一份，不做多版本并列）：

| 文件 | 内容 | 部署位置 |
| --- | --- | --- |
| `sing-box-router-armv7-<ver>` | 裁剪版二进制（UPX 压缩，约 3.9 MiB） | CR8809：`/usr/bin/sing-box`；新机：`/data/sing-box` |
| `sfr-config.example.json` | **脱敏**的 sing-box 配置 | `/etc/sfr-config.json` 或 `/data/sfr-config.json` |
| `etc_init.d_singbox.sh` | init 脚本，**不含任何凭据**，可直接公开 | `/etc/init.d/singbox`（或新机 `/data/singbox.init`） |
| `singbox-boot.sh` | 仅"只读根 + ramfs `/etc`"机型需要，每次开机投放 init | `/data/singbox-boot.sh` |
| `SHA256SUMS` | 上述文件的 sha256 | — |

**不发布未压缩的 `.raw` 二进制**——体积翻 4 倍（16 MiB vs 3.9 MiB），需要时本地
用同一条命令去掉 `upx` 那步重新构建即可。**也不并列发布多套 init**——只出一份，
按机型在文件里改接口名和路径，改动点写在 release 的 `README.md` 里。

init 脚本是纯 bootstrap（iptables 规则 + procd 参数 + 二进制路径），**没有任何凭据**，
唯一带设备指纹的是 IPv6 链路本地地址和文件头注释里的 MAC（见 7.3）。

### 7.2 构建

发布用裁剪版，构建时必须带 `router` tag（`include/registry_router.go` 才会生效）：

```bash
TAGS="with_utls,router"          # with_utls 是 VLESS+REALITY 的硬需求；router 启用裁剪注册表
VER=$(git describe --tags --abbrev=0)
LDF="-X 'github.com/sagernet/sing-box/constant.Version=${VER#v}' -s -w -buildid="

CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -tags "$TAGS" -trimpath -ldflags "$LDF" \
  -o sing-box-router-armv7 ./cmd/sing-box

upx -9 --lzma sing-box-router-armv7
```

对照体积（同一份源码、同一套 flag，只差 `router` tag）：

| 构建 | 原始 | UPX 后 |
| --- | --- | --- |
| 上游全量（`-tags "with_utls"`） | 22.06 MiB | 5.32 MiB |
| **裁剪版（`-tags "with_utls,router"`）** | **15.06 MiB** | **3.91 MiB** |

不带 `router` 编出来的是上游原版，**留着当对照**：线上出问题时先用它复现一次，
能立刻区分是上游行为还是裁剪导致的。

### 7.3 脱敏规则

`sfr-config.json` 里有三类真实凭据，发布前必须替换：

| 字段 | 原值示例 | 替换为 |
| --- | --- | --- |
| `outbounds[].server` | `13.212.241.122` | `203.0.113.1`（RFC 5737 文档用地址） |
| `outbounds[].uuid` | `bf000d23-...` | `00000000-0000-0000-0000-000000000000` |
| `tls.reality.public_key` | `jNXHt1yR...` | `<REALITY_PUBLIC_KEY>` |
| `tls.reality.short_id` | `0123456789abcdef` | `<REALITY_SHORT_ID>` |

**可以保留的**（都是公开信息，留着示例才有意义）：

- `tls.server_name`（REALITY 伪装域名，如 `s3.amazonaws.com`）
- DNS 服务器地址（`1.1.1.1`、`119.29.29.29`）
- 端口号、路由规则、rule-set URL

init 脚本里唯一需要处理的是 **IPv6 链路本地地址**——它由 br-lan 的 MAC 按 EUI-64
推导，会泄露 MAC：

| 字段 | 原值示例 | 替换为 |
| --- | --- | --- |
| v6 DNS 转发目标 | `fe80::d653:2aff:feed:dcdf` | `fe80::<EUI64_OF_BR_LAN_MAC>`，并在注释里写明算法 |

IPv4 的 `192.168.31.1` 保留（小米默认网段），`wl18`/`wl19` 等接口名也保留
（它们是配置里起的名字，不含设备信息）。

### 7.4 发布步骤

```bash
# 1) 定制都提交在 patch 上，先推上去
git switch patch
git push origin patch

# 2) 打发布 tag（格式：上游版本 + 自己的序号）
git tag v1.12.25-router.1
git push origin v1.12.25-router.1

# 3) 按 7.2 构建，按 7.3 脱敏两个文本产物，然后算校验和
sha256sum sing-box-router-armv7 sfr-config.example.json \
          etc_init.d_singbox.sh singbox-boot.sh > SHA256SUMS

# 4) 在 fork 的 Releases 里附上这 5 个文件
```

**发布 tag 与快照 tag 要能区分开**：快照 tag 是裸的 `v1.12.25`（指向上游快照提交），
发布 tag 带 `-router.N` 后缀（指向 `patch` 上的提交）。这样 `git tag` 一眼就能看出
哪个是上游、哪个是自己出的版本。
