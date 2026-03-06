package atproto

// ATProto collection NSIDs for Hypercerts record types.
const (
	// Core claim records (unchanged)
	CollectionActivity        = "org.hypercerts.claim.activity"
	CollectionContributorInfo = "org.hypercerts.claim.contributorInformation"
	CollectionRights          = "org.hypercerts.claim.rights"

	// Context records (renamed from org.hypercerts.claim.*)
	CollectionMeasurement = "org.hypercerts.context.measurement"
	CollectionAttachment  = "org.hypercerts.context.attachment"
	CollectionEvaluation  = "org.hypercerts.context.evaluation"

	// New context record
	CollectionAcknowledgement = "org.hypercerts.context.acknowledgement"

	// New claim record
	CollectionContribution = "org.hypercerts.claim.contribution"

	// Collection (renamed from org.hypercerts.claim.collection)
	CollectionCollection = "org.hypercerts.collection"

	// Funding (unchanged)
	CollectionFundingReceipt = "org.hypercerts.funding.receipt"

	// Work scope records (renamed from org.hypercerts.helper.workScopeTag)
	CollectionWorkScopeTag = "org.hypercerts.workscope.tag"
	CollectionWorkScopeCel = "org.hypercerts.workscope.cel"

	// Badge records
	CollectionBadgeDefinition = "app.certified.badge.definition"
	CollectionBadgeAward      = "app.certified.badge.award"
	CollectionBadgeResponse   = "app.certified.badge.response"

	// Actor records
	CollectionActorProfile      = "app.certified.actor.profile"
	CollectionActorOrganization = "app.certified.actor.organization"

	// Location (unchanged)
	CollectionLocation = "app.certified.location"
)
