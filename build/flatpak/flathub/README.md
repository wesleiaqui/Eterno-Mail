# Flathub Submission

This directory contains the source-build manifest and assets for submitting
Eterno Mail to Flathub after each GitHub release.

## Reproducible Source Build

The stable manifest builds a tagged source revision with vendored Go modules
and generated Node sources. It uses public desktop OAuth configuration from
source; it does not receive OAuth values from GitHub Actions, CI secrets, or
build-time `-ldflags`.

## Prerequisites

Before submitting to Flathub:

1. **Create a GitHub release** from an immutable version tag (for example,
   `v0.1.15`).
2. **Update the stable manifest source** to that exact tag and commit.

## Update `build/flatpak/flathub/io.github.wesleiaqui.eternomail.yml`

After creating a release tag, update the manifest's Git source to the matching
immutable tag and commit, then regenerate vendored dependency sources when the
dependency graph has changed.

```bash
./calculate-hashes.sh v0.1.15
```

The manifest must never follow a moving branch for stable releases.

## Initial Flathub Submission (v0.1.14 - Ready Now!)

### Step 1: Fork flathub/flathub

**Go to**: https://github.com/flathub/flathub/fork

**CRITICAL**: **Uncheck** "Copy the master branch only" - you need the `new-pr` branch!

### Step 2: Clone and Create Submission Branch

```bash
# Clone your fork starting from new-pr branch
git clone --branch=new-pr git@github.com:YOUR_USERNAME/flathub.git
cd flathub

# Create your submission branch
git checkout -b add-eterno-mail new-pr
```

### Step 3: Copy Required Files to Forked Flathub Repo(5 files total)

```bash
cd /path/to/Eterno-Mail
git pull
cd build/flatpak/flathub
# Double check the source manifest and generated dependency sources
./release.sh /path/to/forked/flathub
```

### Step 4: Commit and Push

```bash
cd /path/to/forked/flathub
git add .
git commit -m "Add io.github.wesleiaqui.eternomail"
git push origin add-eterno-mail
```

### Step 5: Create Pull Request

On GitHub, create a pull request:
- **Base repository**: `flathub/flathub`
- **Base branch**: `new-pr` ← **CRITICAL!**
- **Head repository**: `YOUR_USERNAME/flathub`
- **Compare branch**: `add-eterno-mail`
- **Title**: `Add io.github.wesleiaqui.eternomail`

### Step 6: Review Process

Flathub reviewers will:
- Review manifest correctness
- Check metadata completeness
- Request changes if needed

**Common feedback**:
- May ask to restrict `--filesystem=home` to more specific paths
- Verify the source tag, commit, and generated dependency sources match

Comment `bot, build` to trigger a test build once reviewers are satisfied.

### Step 7: Approval & Repository Creation

After approval:
- Flathub creates `flathub/io.github.wesleiaqui.eternomail` repository
- You receive write access invitation (accept within 1 week)
- Must have 2FA enabled on GitHub

## Updating on Flathub (For Future Releases)

After v0.1.14 is on Flathub, for subsequent releases (v0.1.15, v0.1.16, etc.):

```bash
# 1. Create a GitHub release from the new immutable tag

# 2. Get updated manifest
git pull

# 3. Release to Flathub repository (using release.sh helper script)
./release.sh /path/to/flathub/io.github.wesleiaqui.eternomail
# Script automatically copies: manifest, metainfo, desktop, icon, and flathub.json

cd /path/to/flathub/io.github.wesleiaqui.eternomail
git add .
git commit -m "Update to v0.1.15"
git push

# Flathub auto-builds and publishes (no re-review needed!)
```

## Files in This Directory

- `io.github.wesleiaqui.eternomail.yml` - Offline source-build manifest for Flathub
- `io.github.wesleiaqui.eternomail-source.yml` - Legacy local source experiment (not suitable for Flathub)
- `calculate-hashes.sh` - Helper script that automatically updates the manifest with new release hashes
- `release.sh` - Helper script that copies all files to the Flathub repository
- `README.md` - This file

**Files to copy from parent directory for Flathub submission:**
- `../io.github.wesleiaqui.eternomail.metainfo.xml` - AppStream metadata
- `../../linux/io.github.wesleiaqui.eternomail.desktop` - Desktop file
- `../../appicon.png` - Application icon (installed as `io.github.wesleiaqui.eternomail.png`)

## OAuth Credentials

Official desktop OAuth configuration is versioned in
`internal/oauth2/public_clients.go`, so the source manifest can be built
offline without CI secrets or build-time `-ldflags`. Google Desktop client
configuration and Microsoft client IDs are public client configuration; user
tokens and custom credentials are never included in the source or manifest.

## Resources

- [Flathub Submission Guide](https://docs.flathub.org/docs/for-app-authors/submission)
- [App Requirements](https://docs.flathub.org/docs/for-app-authors/requirements)
- [Flathub Review Guidelines](https://docs.flathub.org/docs/for-app-authors/review-guidelines)

## Troubleshooting

**Build fails with "Could not download file"**:
- Ensure release tarballs are publicly accessible on GitHub
- Verify URLs match exactly (case-sensitive)

**SHA256 mismatch error**:
- Re-run `./calculate-hashes.sh` with correct version
- Ensure you're pointing to the correct GitHub release tag

**Permission errors during runtime**:
- Review `finish-args` in manifest
- May need to justify or restrict filesystem access
