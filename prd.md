## PSAP2

One consistent way to decide how important a secure room or area is, how exposed it is, and which protections it must have.

### Actors

There are the following actors on the system:
- Business Owners: own secure rooms, approve acesss requests, request snew rooms and request launches.
- Business Managers: facility managers who assess correct laucnhes and approves them. They are also responsible to shield rooms.
- Security Assessors: business analysts who work for the STAT team. The tier and requirements set what is mandatory. STAT picks the control type, guided by SHIELD drivers.
- Security Implementers: business analysts who install technology in rooms. Usually they are part of the STAT team and normally the same people as security assessors.
- System: coordinates, orchestrates and persists to database.

### Process

1. Request (Business Owners):

Requests a new secure room, a reassessment, or a change to an existing area.

2. Launch (Business Owners):

Answers eight business questions about function, requirements, assets, dependency, recovery, impact and backup.

3. Validate (Business Managers or Assessors):

Checks each answer against evidence and confirms Tier 1–4 or Non-Tier, plus any legal requirement.

4. Shield (Business Managers or Assessors):

Scores nine observable conditions with recorded evidence.

5. Baseline (STAT):

The tier and requirements set what is mandatory. STAT picks the control type, guided by SHIELD drivers.

6. Compliance and investment (Business Managers or Assessors):

Assessor records Met or Gap. Leaders compare same-tier exposure to prioritize spend.

#### Design rules

- LAUNCH captures business impact. The assessor validates it. LAUNCH does not ask about cameras, readers, alarms or barriers.
- SHIELD captures observable exposure only. It is not a pass/fail checklist of installed controls.
- The tier changes only through a formal reassessment of business criticality, never because of SHIELD or installed controls.
- Confirmed legal or regulatory requirements are added to the baseline as mandatory parameters.

### Tiers

Four fixed tiers, plus Non-Tier. The tier definitions are governing; the score supports consistent assignment.

- Tier 1: Working score 4.00–5.00 - Severe impact
Any resource whose loss, damage or compromise results in a degree of impact that critically changes and causes loss or irreversible business capabilities.

- Tier 2: Working score 3.00–3.99 - Significant impact
Any resource whose loss, damage or compromise results in a degree of impact that damages and degrades business capabilities.

- Tier 3: Working score 2.00–2.99 - Moderate impact
Any resource whose loss, damage or compromise results in a degree of impact that threatens to damage and degrade business capabilities.

- Tier 4: Working score 1.00–1.99 - Minor impact
Any resource whose loss, damage or compromise results in a degree of impact that may disrupt business capabilities.

### Launch

Answer for the actual room or area. Security will validate your answers and confirm the result. This page runs entirely in your browser and saves nothing.

These are the 8 Question Categories and their questions. Some answers are not simple Yes or No.

- Room Function: 
    - What functions does this room or area support? Select all that apply.
        - Safekeeping of valuables such as currency or IT equipment
        - High-value asset storage
        - Data transmission / network infrastructure
        - Data storage
        - Sensitive business operations
        - Building infrastructure such as water, electrical or HVAC
        - Executive leadership area
        - Physical security operations
        - Life-safety systems
        - Other
- Regularory Requirement: 
    - Are there any legal, regulatory, or business requirements that require physical security measures for this room or area?
        - Yes: Requirement identified by business/SME?
        - No: No known requirement?
        - Unknown: Needs Security/Compliance validation?
- Asset Type: 
    - What type of assets are housed in this room or area? Answer for the highest-criticality asset, system or information actually housed in the room.
        - General: General office equipment or supplies
        - Department-specific: Department-specific equipment, records or resources
        - Sensitive: Shared business systems or sensitive information
        - Regulatory: Association systems, security systems or regulated information
        - Mission-critical infrastructure: life-safety systems or highly sensitive assets
- Routine Access: 
    - How limited is the legitimate business population requiring access to this asset?
        - Broad business population: no access restrictions?
        - Defined business population: all badged employees and contractors?
        - Limited business population: badged employees only?
        - Restricted access population: access limited to a small number of specifically approved individuals
        - Highly restricted access only: normally limited to very few individuals and may require approval for a specific purpose each time
- Business Dependency: 
    - How dependent are daily business operations on this room or area?
        - Minimal: convenience function only
        - Limited: upports a single team or department
        - Moderate: supports multiple departments or a business function
        - Significant: supports critical business operations or major site functions
        - Severe: supports Association-wide operations or could cause major business interruption
- Recovery Capability: 
    - If this room or area became unavailable, how quickly could operations be restored using existing recovery plans, redundancies or alternate locations?
        - Within hours
        - Within 1 day
        - Within 1 week
        - Within 1 month
        - More than 1 month or no practical recovery option
- Unauthorized Access Impact:
    - What could be the impact if assets, systems, information or equipment in this room were accessed by an unauthorized person?
        - No meaningful impact
        - Limited ompact to a small workshop
        - Modederate impact to a department or business function
        - Significant impact to multiple departments, a site or customers
        - Severe enterprise, regulatory, financial or reputational impact
- Backup / Alternate Resources:
    - Are backup systems or alternate resources available if a primary system or component in this room fails?
        - Full redundancy: continuity with little or no interruption
        - High redundancy: backup available with minimal downtime
        - Moderate redundancy: backups exist, but single points of failure remain
        - Limited redundancy: some backup capability exists but recovery requires significant effort
        - No redundancy: critical functions depend on a single point of failure
        - Not applicable: no function requiring redundancy

#### LAUNCH weighted score

Score = Q3 × 0.20 + Q5 × 0.25 + Q6 × 0.20 + Q7 × 0.25 + Q8 × 0.10. Q1, Q2 and Q4 carry no weight. Every question scores 1–5 in the order the answers appear, and higher always means higher business criticality.

### Shield

Score observable conditions using the written anchors. Record the evidence for every answer. SHIELD never changes the tier or waives the baseline. Collectively they capture exterior and environmental exposure that can require alarms, cameras, hardened construction, standoff, barriers, fire/life-safety coordination or other special controls.

These are the 9 Question Categories and their questions. Some answers are not simple Yes or No.

- Physical Proximity: 
    - Counting from the nearest uncontrolled or public point, how many access-controlled zone boundaries must be crossed to reach the room or area? (Evidence is required for this question):
        - Must cross 3 or more access-controlled zone boundaries
        - Must cross 2 access-controlled zone boundaries
        - Must cross 1 access-controlled zone boundary
        - Can reach the immediate area without crossing a zone boundary, but not directly from a public area
        - Can directly approach the asset from a public or unrestricted area
- Entry: 
    - How difficult would it be for an unauthorized individual to gain entry through the room or area’s own boundary? (Evidence is required for this question):
        - Entry requires defeating 3 or more independent barriers at the room/area boundary
        - Entry requires defeating 2 independent barriers at the room/area boundary
        - Entry requires defeating 1 hardened or rated barrier (per job-aid barrier table)
        - Entry requires bypassing 1 standard, unrated barrier (standard door, gate or similar)
        - No physical barrier prevents entry
- Time: 
    - How long does it take to walk from the nearest uncontrolled approach point to the room or area’s outermost door or opening? (Record a timed walk) (Evidence is required for this question):
        - More than 10 minutes
        - 5 to 10 minutes
        - 2 to 5 minutes
        - 30 seconds to 2 minutes
        - Less than 30 seconds
Skill:
    - What level of skill, tools, inside knowledge, or specialized equipment would be needed to bypass or defeat the room or area’s physical boundary? (Evidence is required for this question):
        - Requires specialized technical knowledge and equipment not commonly available
        - Requires specialized equipment or documented technical knowledge
        - Requires commercially available tools and basic knowledge of the barrier or control
        - Ordinary hand tools or simple manipulation could defeat the boundary
        - No tools or specialized knowledge needed; boundary is easily bypassed
Visual Exposure:
    - How easily can the asset be seen and observed in its environment? (Evidence is required for this question)
        - Not visible from any uncontrolled area
        - Visible only from a restricted or controlled adjacent area
        - Partially visible from an uncontrolled area
        - Fully visible from an uncontrolled (non-public) area
        - Fully visible from a public or unrestricted area
Recog:
    - How easily can the asset’s purpose, value or importance be understood by someone unfamiliar with it? (Evidence is required for this question)
        - Purpose cannot reasonably be determined from visible characteristics
        - Purpose can only be determined with specialized or internal knowledge
        - General function can be inferred, but specific purpose/value is not apparent
        - Purpose is apparent from signage, equipment, layout or routine activity
        - Purpose and security significance are explicitly identified or obvious
Information Availability:
    - How readily available is information about the asset’s characteristics, location or function to the general public?  (Evidence is required for this question)
        - No public information identifies the asset or its location
        - General facility information is public, but the asset is not identified
        - Asset type/function is publicly identifiable, but location/details are not
        - Asset location or operational details are publicly available
        - Detailed location, function, configuration or access information is publicly available
Human / Operational Exposure:
    - Which people not permanently assigned to the room or area enter or work in the immediate area, and how often? (Evidence is required for this question)
        - No non-assigned persons, or exception-only (escorted) access
        - Non-assigned persons enter less than monthly
        - Non-assigned employees or contractors enter monthly to weekly
        - Contractors or service personnel enter daily or routinely
        - Visitors, customers/members or the public enter daily or routinely
Exterior / Environmental Exposure:
    - Does the room or area have direct exposure to exterior, uncontrolled or environmental conditions that affect required protection? (Evidence is required for this question)
        - Fully interior; no uncontrolled adjacency or exterior exposure
        - Adjacent to lower-controlled interior space only
        - Exterior wall, service penetration or nearby utility/environmental exposure
        - Accessible exterior opening, service route, dock or equipment opening
        - Direct public exterior door, vehicle-accessible boundary or high-risk environmental exposure

#### Scoring rule

Score 1 = lowest exposure; 5 = highest. Record the evidence/source. Select the highest applicable exposure condition actually present and attach a photo or drawing reference for the condition that drove the score.

### User Interface

#### Demo Version

The initial user interface is for demo purpose so we can convey the concepts to stake holders. For this demo version, I would like a React interface that calls the API layer. No need to worry about auth/authz for now as these will change dramatically once we are in our real environment.

Given that this is a demo version, I would like to show how each actor access looks like:
- Business Owners: own secure rooms, approve acesss requests, request snew rooms and request launches.
- Business Managers: facility managers who assess correct laucnhes and approves them. They are also responsible to shield rooms.
- Security Assessors: business analysts who work for the STAT team. The tier and requirements set what is mandatory. STAT picks the control type, guided by SHIELD drivers.
- Security Implementers: business analysts who install technology in rooms. Usually they are part of the STAT team and normally the same people as security assessors.

But since we have not implemented the last 2 actors, let us leave them off.

##### Business Owners

- Show a list of assets they own, a way to request a new asset and a way to re-assess existing ones.
- Show a list of pending launches they need to complete. 
- Pending launches show the question categories on the left and an indicator whether they are completed or not. Once a category is clicked, the questions do open and the business owner can answer them as needed. Please make the navigation among the different categories simple and intuitive (perhaps allow arrow keys to navigate in addition to clicks). 
- Business Owners can view the detail of the completed or in-progress assets, associated tier, associated launches and associated guides. 

