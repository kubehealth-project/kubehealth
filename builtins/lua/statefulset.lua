local desired = 1
if obj.spec ~= nil and obj.spec.replicas ~= nil then
  desired = obj.spec.replicas
end

local status = obj.status or {}
local ready = status.readyReplicas or 0
local available = status.availableReplicas or 0
local updated = status.updatedReplicas or 0

local availability = "Unavailable"
local availabilityMessage = "No StatefulSet replicas are available"
if desired == 0 then
  availability = "NotApplicable"
  availabilityMessage = "StatefulSet is scaled to zero"
elseif available >= desired then
  availability = "Available"
  availabilityMessage = string.format("%d replicas are available", available)
elseif available > 0 then
  availability = "PartiallyAvailable"
  availabilityMessage = string.format("%d of %d desired replicas are available", available, desired)
end

local function assessment(reconciliation, message)
  return {
    reconciliation = reconciliation,
    availability = availability,
    lifecycle = "Active",
    reconciliationMessage = message,
    availabilityMessage = availabilityMessage
  }
end

if ready < desired then
  return assessment("InProgress", string.format("Ready: %d/%d", ready, desired))
end

local strategy = "RollingUpdate"
if obj.spec ~= nil and obj.spec.updateStrategy ~= nil and obj.spec.updateStrategy.type ~= nil then
  strategy = obj.spec.updateStrategy.type
end

if strategy == "OnDelete" then
  return assessment("Reconciled", "StatefulSet is ready with OnDelete updates")
end

local partition = 0
if obj.spec ~= nil and obj.spec.updateStrategy ~= nil and
   obj.spec.updateStrategy.rollingUpdate ~= nil and
   obj.spec.updateStrategy.rollingUpdate.partition ~= nil then
  partition = obj.spec.updateStrategy.rollingUpdate.partition
end

local expectedUpdated = desired - partition
if expectedUpdated < 0 then
  expectedUpdated = 0
end
if updated < expectedUpdated then
  return assessment("InProgress", string.format("Updated: %d/%d", updated, expectedUpdated))
end

if partition == 0 and status.currentRevision ~= nil and status.updateRevision ~= nil and
   status.currentRevision ~= status.updateRevision then
  return assessment("InProgress", string.format("Waiting for revision %s", status.updateRevision))
end

return assessment("Reconciled", "StatefulSet is reconciled")
