# Flathub Submission

This directory contains the assets for submitting Eterno Mail to Flathub using **pre-built binaries** (extra-data approach) after each GitHub release.

## Why Pre-Built Binaries?

Eterno Mail uses the extra-data approach (similar to Discord, Spotify) because OAuth credentials are embedded at build time in GitHub Actions. This allows users to have Gmail/Outlook OAuth working out-of-the-box without exposing secrets to Flathub's build infrastructure.

## Prerequisites

Before submitting to Flathub:

1. **Create a GitHub release** with version tag (e.g., `v0.1.15`)
2. **Ensure release includes**:
   - `aerion-v0.1.13-linux-x86_64.tar.gz` (x86_64 binary with OAuth credentials)
   - `aerion-v0.1.13-linux-aarch64.tar.gz` (aarch64 binary with OAuth credentials)

## Wait for GitHub Actions to build and update `build/flatpak/flathub/io.github.wesleiaqui.EternoMail.yml`

Github Actions will use the provided script to calculate SHA256 hashes and automatically update the manifest:

```bash
./calculate-hashes.sh v0.1.15
```

This will:
- Download the release tarballs from GitHub
- Calculate SHA256 hashes and file sizes
- Automatically update `io.github.wesleiaqui.EternoMail.yml` with new values
- Create a backup of the original file
- Commit the new `io.github.wesleiaqui.EternoMail.yml` with the commit message, "v0.1.15 - Flathub Submission"

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
# Double check the new extradata file
./release.sh /path/to/forked/flathub
```

### Step 4: Commit and Push

```bash
cd /path/to/forked/flathub
git add .
git commit -m "Add io.github.wesleiaqui.EternoMail"
git push origin add-eterno-mail
```

### Step 5: Create Pull Request

On GitHub, create a pull request:
- **Base repository**: `flathub/flathub`
- **Base branch**: `new-pr` ← **CRITICAL!**
- **Head repository**: `YOUR_USERNAME/flathub`
- **Compare branch**: `add-eterno-mail`
- **Title**: `Add io.github.wesleiaqui.EternoMail`

### Step 6: Review Process

Flathub reviewers will:
- Review manifest correctness
- Check metadata completeness
- Request changes if needed

**Common feedback**:
- May ask to restrict `--filesystem=home` to more specific paths
- Verify extra-data checksums match

Comment `bot, build` to trigger a test build once reviewers are satisfied.

### Step 7: Approval & Repository Creation

After approval:
- Flathub creates `flathub/io.github.wesleiaqui.EternoMail` repository
- You receive write access invitation (accept within 1 week)
- Must have 2FA enabled on GitHub

## Updating on Flathub (For Future Releases)

After v0.1.14 is on Flathub, for subsequent releases (v0.1.15, v0.1.16, etc.):

```bash
# 1. Create GitHub release with new binaries (GitHub Actions does this)

# 2. Get updated manifest
git pull

# 3. Release to Flathub repository (using release.sh helper script)
./release.sh /path/to/flathub/io.github.wesleiaqui.EternoMail
# Script automatically copies: manifest, metainfo, desktop, icon, and flathub.json

cd /path/to/flathub/io.github.wesleiaqui.EternoMail
git add .
git commit -m "Update to v0.1.15"
git push

# Flathub auto-builds and publishes (no re-review needed!)
```

## Files in This Directory

- `io.github.wesleiaqui.EternoMail.yml` - Flatpak manifest using pre-built binaries (main manifest for Flathub)
- `io.github.wesleiaqui.EternoMail-source.yml` - Alternative manifest (builds from source, not used for Flathub)
- `calculate-hashes.sh` - Helper script that automatically updates the manifest with new release hashes
- `release.sh` - Helper script that copies all files to the Flathub repository
- `README.md` - This file

**Files to copy from parent directory for Flathub submission:**
- `../io.github.wesleiaqui.EternoMail.metainfo.xml` - AppStream metadata
- `../../linux/io.github.wesleiaqui.EternoMail.desktop` - Desktop file
- `../../appicon.png` - Application icon (installed as `io.github.wesleiaqui.EternoMail.png`)

## OAuth Credentials

With the extra-data approach, OAuth credentials are **already embedded** in the pre-built binaries. Users get working Gmail/Outlook OAuth out-of-the-box without any additional configuration.

## Resources

- [Flathub Submission Guide](https://docs.flathub.org/docs/for-app-authors/submission)
- [App Requirements](https://docs.flathub.org/docs/for-app-authors/requirements)
- [Flathub Review Guidelines](https://docs.flathub.org/docs/for-app-authors/review-guidelines)
- [Extra Data Documentation](https://docs.flatpak.org/en/latest/flatpak-builder-command-reference.html#extra-data-sources)

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
