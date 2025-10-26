# Image Override Approach - Proposal

## Problem
AWS ECS does not support image override via `ContainerOverride`. The image must be specified in the task definition itself.

## Options Comparison

### Option 1: No Image Override ❌
**Remove --image flag entirely**

Users must use base image (ubuntu:22.04) or pre-configure via CloudFormation.

**Pros:**
- Simplest architecture
- No dynamic resource creation
- No additional IAM permissions needed

**Cons:**
- Much less flexible
- Can't quickly test different tools
- Defeats purpose of "use any Docker image"

**Verdict:** Too limiting for the project's goals

---

### Option 2: Pre-register Images During Init 🤔
**Ask user for images during `mycli init`, register them upfront**

```bash
$ mycli init
...
→ Which Docker images will you use? (comma-separated)
Images: hashicorp/terraform:1.6, python:3.11, node:18

✓ Registered: mycli-task-terraform
✓ Registered: mycli-task-python  
✓ Registered: mycli-task-node
```

Add command: `mycli register-image <name> <image>`

**Pros:**
- No runtime overhead
- Predictable resource creation
- Can view all task definitions in CloudFormation/Console

**Cons:**
- Requires upfront planning
- Still creates multiple task definitions
- Need extra command to add images later
- User can't just run `mycli exec --image=foo:bar` on the fly

**Verdict:** Better than Option 1, but still limiting

---

### Option 3: Dynamic Registration (Current Implementation) ✅
**Register task definitions on-demand when image specified**

**Pros:**
- Maximum flexibility - use any image anytime
- Automatic caching (checks if exists before creating)
- No upfront configuration needed
- Matches the "just works" philosophy

**Cons:**
- Creates many task definition revisions over time
- First execution with new image slightly slower (~100-200ms)
- More IAM permissions needed
- Less "clean" architecture

**Improvements we could make:**
1. Add task definition cleanup/pruning
2. Better naming: `mycli-task-<short-hash>` already does this
3. Document in logs: "Using cached task definition" vs "Registering new task definition"
4. Add `mycli list-images` command to show registered task definitions

**Verdict:** Most flexible, aligns with project goals

---

### Option 4: Hybrid Approach 🎯
**Combine pre-registration with dynamic fallback**

1. During `mycli init`, ask for common images (optional)
2. Create named task definitions: `mycli-task-terraform`, `mycli-task-python`
3. At runtime:
   - If `--image=terraform` → use `mycli-task-terraform` (named lookup)
   - If `--image=hashicorp/terraform:1.6` → use dynamic registration with hash

**Pros:**
- Best of both worlds
- Named shortcuts for common images
- Still allows any arbitrary image
- Clean for common cases, flexible for edge cases

**Cons:**
- Most complex to implement
- Potentially confusing (two lookup mechanisms)

---

## Recommendation

**Option 3 (Dynamic Registration)** with improvements:

1. **Keep the current implementation** - it works and provides maximum flexibility
2. **Add transparency** - log when creating vs reusing task definitions
3. **Add management commands:**
   ```bash
   mycli list-task-definitions              # Show all mycli task definitions
   mycli cleanup-task-definitions --older-than=30d  # Prune old ones
   ```
4. **Document the trade-off** in ARCHITECTURE.md clearly

**Why:** This aligns with mycli's philosophy of "just works with any image". Users don't need to pre-plan or configure - they can just run commands with any image on-demand.

**AWS keeps task definition revisions forever** (they're free, just metadata), so the "pollution" concern is minimal. We can provide cleanup tools for users who care.

## Alternative: If Dynamic is Truly Unacceptable

If dynamic registration is a dealbreaker, **Option 2 (Pre-register)** would be the fallback. But it significantly limits the tool's flexibility and requires more user friction upfront.
