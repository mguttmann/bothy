-- Reads of the export of one person (docs/spec/20-privacy.md section 5.2, M7-06): the whole
-- history, revoked grants and removed memberships included, because the person has a right to
-- know which access existed and when it ended.

-- name: ListGrantsOfPrincipalHistory :many
-- Every grant whose subject is the principal, revoked or not; grants to a group of the person are
-- grants of the group and not listed.
SELECT sqlc.embed(g)
FROM grants g
WHERE g.subject_principal_id = @principal_id
ORDER BY g.id;

-- name: ListMembershipsOfUserHistory :many
-- Every membership of the person, removed or not, with the row of its group (deleted or not).
SELECT sqlc.embed(m), sqlc.embed(g)
FROM group_memberships m
JOIN groups g ON g.id = m.group_id
WHERE m.user_id = @user_id
ORDER BY m.id;
