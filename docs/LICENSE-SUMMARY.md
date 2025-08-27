# License Summary - AGPL-3.0

## Quick Reference

**License**: GNU Affero General Public License v3.0 (AGPL-3.0)  
**SPDX Identifier**: `AGPL-3.0-or-later`  
**Full Text**: See [LICENSE](LICENSE) file  
**Official Source**: https://www.gnu.org/licenses/agpl-3.0.html

## What is AGPL-3.0?

The AGPL-3.0 is a strong copyleft license that extends the GPL-3.0 to cover network/SaaS usage. It ensures that any modifications to the software remain open source, even when used as a network service.

## Key Terms at a Glance

### ✅ Permissions (What You CAN Do)
- **Commercial use**: Use the software commercially
- **Modification**: Modify the source code
- **Distribution**: Distribute the software
- **Patent use**: Use patent claims of contributors
- **Private use**: Use privately without distributing

### ⚠️ Conditions (What You MUST Do)
- **Disclose source**: Source code must be made available
- **License and copyright notice**: Include original copyright and license
- **State changes**: Document modifications made to the code
- **Same license**: Distribute under the same AGPL-3.0 license
- **Network use is distribution**: Providing network access triggers source disclosure

### ❌ Limitations (What is NOT Covered)
- **Liability**: No liability for damages
- **Warranty**: No warranty provided

## Important Implications

### For Users Running the Software

#### Internal/Private Use Only
- ✅ Can use freely within your organization
- ✅ Can modify for internal needs
- ✅ No obligation to share changes if not distributed

#### Running as a Service (SaaS/Network)
- ⚠️ Must provide source code to users upon request
- ⚠️ Must include all modifications
- ⚠️ Must provide installation instructions
- ⚠️ Applies even if you don't distribute the binary

#### Distributing the Software
- ⚠️ Must provide complete source code
- ⚠️ Must use AGPL-3.0 license
- ⚠️ Must preserve all copyright notices
- ⚠️ Must document your changes

### For Contributors
- Your contributions will be licensed under AGPL-3.0
- You grant patent rights for your contributions
- You retain copyright on your contributions
- Your work will always remain open source

### For Commercial Use

#### ✅ You CAN:
- Use the software in commercial products
- Charge for services using the software
- Sell support and consulting
- Use internally without restrictions

#### ⚠️ You MUST:
- Keep any modifications open source
- Provide source code if offering as a service
- Use AGPL-3.0 for any derivative works
- Give credit to original authors

## Common Scenarios

### Scenario 1: Using in Your Kubernetes Cluster
**Private Internal Use**
- ✅ Deploy to your private cluster
- ✅ Modify for your needs
- ✅ No source disclosure required

**Public Cloud Service**
- ⚠️ Must provide source code link to users
- ⚠️ Must include your modifications

### Scenario 2: Creating a Fork
- ⚠️ Must keep AGPL-3.0 license
- ⚠️ Must preserve copyright notices
- ⚠️ Must document changes
- ✅ Can add your copyright for modifications

### Scenario 3: Including in a Larger Project
- ⚠️ May need to license entire project as AGPL-3.0
- ⚠️ Consult legal counsel for mixed licensing
- ✅ Can keep separate if loosely coupled

## FAQ

### Q: Can I use this in my proprietary software?
**A**: Yes, but with conditions. If you distribute or provide network access to the software, you must provide source code and license your modifications under AGPL-3.0.

### Q: What's the difference between AGPL and GPL?
**A**: AGPL adds the "network use is distribution" clause. With GPL, running software as a service doesn't trigger source disclosure. With AGPL, it does.

### Q: Can I use this commercially?
**A**: Yes! Commercial use is explicitly permitted. You can charge for the software, support, or services. You just must keep it open source.

### Q: Do I need to open-source my entire application?
**A**: Only if your application is a derivative work. If the right-sizer runs as a separate service/container, your application may not be affected. Consult legal counsel for specific cases.

### Q: Can I use this in a closed-source Kubernetes platform?
**A**: You can run it as a separate operator/service. If you modify and integrate it deeply into your platform, the AGPL provisions apply to those components.

## Compliance Checklist

When using right-sizer, ensure:

- [ ] Keep the LICENSE file intact
- [ ] Preserve all copyright notices
- [ ] Document your modifications
- [ ] If distributing: provide source code
- [ ] If running as service: provide source access to users
- [ ] Use AGPL-3.0 for derivative works
- [ ] Include NOTICE file with attributions

## Need Legal Advice?

This summary is for informational purposes only and does not constitute legal advice. For specific situations, consult with a legal professional familiar with open source licensing.

## Additional Resources

- [AGPL-3.0 Full Text](https://www.gnu.org/licenses/agpl-3.0.html)
- [FSF AGPL FAQ](https://www.gnu.org/licenses/agpl-3.0.html#section13)
- [SPDX License Information](https://spdx.org/licenses/AGPL-3.0-or-later.html)
- [Choose a License Comparison](https://choosealicense.com/licenses/agpl-3.0/)

---

*Last Updated: 2024*  
*This is a simplified summary. Always refer to the full [LICENSE](LICENSE) for authoritative terms.*