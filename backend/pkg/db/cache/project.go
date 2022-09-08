package cache

import (
	. "openreplay/backend/pkg/db/types"
	"time"
)

type ProjectMeta struct {
	*Project
	expirationTime time.Time
}

func (c *cacheImpl) GetProjectByKey(projectKey string) (*Project, error) {
	pmInterface, found := c.projectsByKeys.Load(projectKey)
	if found {
		if pm, ok := pmInterface.(*ProjectMeta); ok {
			if time.Now().Before(pm.expirationTime) {
				return pm.Project, nil
			}
		}
	}

	p, err := c.getProjectByKey(projectKey)
	if err != nil {
		return nil, err
	}
	//c.projects[ p.ProjectID ] = &ProjectMeta{ p, time.Now().Add(c.projectExpirationTimeout) }
	c.projectsByKeys.Store(projectKey, p)
	return p, nil
}

func (c *cacheImpl) GetProject(projectID uint32) (*Project, error) {
	if c.projects[projectID] != nil &&
		time.Now().Before(c.projects[projectID].expirationTime) {
		return c.projects[projectID].Project, nil
	}
	p, err := c.getProject(projectID)
	if err != nil {
		return nil, err
	}
	c.projects[projectID] = &ProjectMeta{p, time.Now().Add(c.projectExpirationTimeout)}
	//c.projectsByKeys.Store(p.ProjectKey, c.projects[ projectID ])
	return p, nil
}

func (c *cacheImpl) getProjectByKey(projectKey string) (*Project, error) {
	p := &Project{ProjectKey: projectKey}
	if err := c.conn.QueryRow(`
		SELECT max_session_duration, sample_rate, project_id
		FROM projects
		WHERE project_key=$1 AND active = true
	`,
		projectKey,
	).Scan(&p.MaxSessionDuration, &p.SampleRate, &p.ProjectID); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *cacheImpl) getProject(projectID uint32) (*Project, error) {
	p := &Project{ProjectID: projectID}
	if err := c.conn.QueryRow(`
		SELECT project_key, max_session_duration, save_request_payloads,
			metadata_1, metadata_2, metadata_3, metadata_4, metadata_5,
			metadata_6, metadata_7, metadata_8, metadata_9, metadata_10
		FROM projects
		WHERE project_id=$1 AND active = true
	`,
		projectID,
	).Scan(&p.ProjectKey, &p.MaxSessionDuration, &p.SaveRequestPayloads,
		&p.Metadata1, &p.Metadata2, &p.Metadata3, &p.Metadata4, &p.Metadata5,
		&p.Metadata6, &p.Metadata7, &p.Metadata8, &p.Metadata9, &p.Metadata10); err != nil {
		return nil, err
	}
	return p, nil
}
