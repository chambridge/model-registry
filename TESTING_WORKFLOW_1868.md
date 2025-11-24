# Testing Guide for Issue #1868 Workflow Changes

This guide explains how to test the container image publishing workflow changes before merging to main.

## Platform Support Question

### What happened to s390x and ppc64le?

**Short answer**: They were reduced from the default but were never actually used in the main workflow.

**Detailed explanation**:

- **Main branch workflow** sets: `PLATFORMS: linux/arm64,linux/amd64` (only 2 platforms)
- **build_deploy.sh script** has a fallback: `PLATFORMS="${PLATFORMS:-linux/arm64,linux/amd64,linux/s390x,linux/ppc64le}"`
- **Your new workflow**: Directly specifies `platforms: linux/arm64,linux/amd64`

The 4-platform support existed in the script's default, but the workflow never used it. You're maintaining the same 2-platform support that was actually being used.

### Should you add them back?

**Recommendation**: Only add them if you have enterprise users who need IBM Z (s390x) or IBM Power (ppc64le).

**If you want to add them back**, change line 113 in the workflow:

```yaml
# Before:
platforms: linux/arm64,linux/amd64

# After (all 4 platforms):
platforms: linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
```

**Trade-offs**:
- ✅ **Pro**: Broader platform support for enterprise users
- ⚠️ **Con**: Significantly longer build times (~4x longer)
- ⚠️ **Con**: Higher CI costs
- ⚠️ **Con**: More complex debugging if platform-specific issues arise

## Testing Approaches

### Option 1: Test on Your Fork (Recommended for Full Testing)

A test workflow file has been created: `.github/workflows/build-and-push-image-test.yml`

**Steps:**

1. **Ensure you have a fork** of the repository on GitHub

2. **Push your branch to your fork**:
   ```bash
   git push origin chore/1868-modernize-container-image-publishing
   ```

3. **The test workflow will automatically trigger** because it's configured to run on this branch

4. **Monitor the workflow**:
   - Go to: `https://github.com/YOUR_USERNAME/model-registry/actions`
   - Watch the "Container image build and tag (TEST)" workflow

5. **Verify the results**:
   - Check that the build completes successfully
   - Note the image digest from the workflow logs
   - The image will be pushed to: `ghcr.io/YOUR_USERNAME/model-registry/server:test-XXXXXX`

6. **Test verification commands** (install cosign first: `brew install cosign` on macOS):

   ```bash
   # Replace YOUR_USERNAME and TAG with your values
   IMAGE="ghcr.io/YOUR_USERNAME/model-registry/server:test-XXXXXX"

   # Verify signature
   cosign verify \
     --certificate-identity-regexp=github \
     --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
     "${IMAGE}"

   # Verify SBOM attestation
   cosign verify-attestation \
     --type spdx \
     --certificate-identity-regexp=github \
     --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
     "${IMAGE}"

   # Download and view SBOM
   cosign download attestation \
     --predicate-type=https://spdx.dev/Document \
     "${IMAGE}" | jq .

   # Verify SLSA provenance
   cosign verify-attestation \
     --type slsaprovenance \
     --certificate-identity-regexp=github \
     --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
     "${IMAGE}"
   ```

7. **Delete the test workflow** before creating your PR:
   ```bash
   git rm .github/workflows/build-and-push-image-test.yml
   git commit -m "chore: remove test workflow file"
   ```

### Option 2: Manual Workflow Dispatch (Quick Test)

The test workflow includes `workflow_dispatch` trigger, allowing manual runs:

1. Push the test workflow to your fork
2. Go to: `https://github.com/YOUR_USERNAME/model-registry/actions`
3. Select "Container image build and tag (TEST)" workflow
4. Click "Run workflow" button
5. Select your branch and click "Run workflow"

### Option 3: Test Using Act (Local Testing - Limited)

**Note**: This tests workflow syntax but won't test signing/attestation (requires GitHub OIDC).

```bash
# Install act (https://github.com/nektos/act)
brew install act

# Test the workflow locally (dry-run)
act push --dryrun -W .github/workflows/build-and-push-image-test.yml

# Test with actual execution (won't push or sign)
act push -W .github/workflows/build-and-push-image-test.yml
```

**Limitations**:
- Can't test keyless signing (requires GitHub OIDC)
- Can't test actual image push (requires credentials)
- Good for syntax validation only

### Option 4: Create a Test Tag on Your Fork

If you want to test the tag-based workflow:

```bash
# Create a test tag
git tag v0.0.0-test-1868
git push origin v0.0.0-test-1868

# Watch the workflow run
# Image will be: ghcr.io/YOUR_USERNAME/model-registry/server:v0.0.0-test-1868

# Clean up tag after testing
git tag -d v0.0.0-test-1868
git push origin :refs/tags/v0.0.0-test-1868
```

## What to Verify During Testing

### 1. Build Success
- ✅ Multi-arch build completes without errors
- ✅ Image is pushed to registry
- ✅ All platforms (arm64, amd64) are built

### 2. SBOM Generation
- ✅ Anchore SBOM is generated successfully
- ✅ SBOM file exists before attestation step
- ✅ No duplicate SBOMs in registry

### 3. Signing
- ✅ Cosign signing completes successfully
- ✅ Signature is verifiable with cosign
- ✅ Certificate chain is valid

### 4. Attestation
- ✅ SBOM attestation is created and attached
- ✅ SBOM attestation is verifiable
- ✅ SLSA provenance attestation is created (by docker/build-push-action)
- ✅ Provenance is verifiable

### 5. Metadata
- ✅ Tags are created correctly (main, latest, sha-based for main branch)
- ✅ Labels are applied to image
- ✅ Version tags work correctly for tag triggers

### 6. Error Handling
- ✅ Workflow fails gracefully if digest is missing
- ✅ Clear error messages in logs
- ✅ SBOM file validation works

## Testing Checklist

Before creating your PR, ensure you've tested:

- [ ] Test workflow runs successfully on fork
- [ ] Image builds for both arm64 and amd64 platforms
- [ ] SBOM is generated and can be downloaded
- [ ] Image signature verifies successfully
- [ ] SBOM attestation verifies successfully
- [ ] SLSA provenance attestation verifies successfully
- [ ] Tags are created as expected
- [ ] Error handling triggers correctly (optional: modify workflow to test)
- [ ] Test workflow file is removed before PR
- [ ] Documentation in workflow header is accurate

## After Testing - PR Preparation

Once testing is complete:

1. **Remove the test workflow**:
   ```bash
   git rm .github/workflows/build-and-push-image-test.yml
   git rm TESTING_WORKFLOW_1868.md  # This file
   git add .github/workflows/build-and-push-image.yml
   git commit -m "chore(ci): modernize container image publishing workflow per #1868"
   ```

2. **Create your PR** with the following information:
   - Link to issue #1868
   - Summary of changes (use the review document as reference)
   - Evidence of testing (screenshots, verification commands output)
   - Note about platform support (2 platforms: arm64, amd64)

3. **Include verification examples** in PR description:
   ```markdown
   ## Testing Evidence

   Tested on fork: [link to workflow run]

   Image: ghcr.io/YOUR_USERNAME/model-registry/server:test-XXXXXX

   Verification results:
   - ✅ Signature verified
   - ✅ SBOM attestation verified
   - ✅ SLSA provenance verified

   [Include screenshots or command output]
   ```

## Troubleshooting

### "Permission denied" when pushing to ghcr.io
- Check that your fork has package write permissions enabled
- Go to: Settings → Actions → General → Workflow permissions
- Enable "Read and write permissions"

### "OIDC token not found" during signing
- This only works in GitHub Actions, not locally
- Ensure you're running on GitHub Actions, not with `act`

### Build timeout or very slow
- Multi-arch builds are slow (especially first time)
- 60-minute timeout should be sufficient
- Use GitHub Actions cache to speed up subsequent builds

### Verification fails with "no matching signatures"
- Ensure you're using the correct certificate-identity-regexp
- Check that the image tag/digest is correct
- Verify you're using HTTPS for the OIDC issuer URL

## Questions?

If you encounter issues during testing:
1. Check GitHub Actions logs for detailed error messages
2. Review the workflow file for correct syntax
3. Ensure all secrets and permissions are properly configured
4. Ask for help in the PR or issue #1868

## Clean Up After Testing

Don't forget to:
- Delete test images from your GHCR (to save space)
- Remove test tags from your fork
- Remove the test workflow file before submitting PR
- Delete this testing guide before submitting PR
