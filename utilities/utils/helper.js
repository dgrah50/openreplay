let PROJECT_KEY_LENGTH = parseInt(process.env.PROJECT_KEY_LENGTH) || 20;
let debug = process.env.debug === "1" || false;
const extractPeerId = (peerId) => {
    let splited = peerId.split("-");
    if (splited.length !== 2) {
        debug && console.error(`cannot split peerId: ${peerId}`);
        return {};
    }
    if (PROJECT_KEY_LENGTH > 0 && splited[0].length !== PROJECT_KEY_LENGTH) {
        debug && console.error(`wrong project key length for peerId: ${peerId}`);
        return {};
    }
    return {projectKey: splited[0], sessionId: splited[1]};
};
const request_logger = (identity) => {
    return (req, res, next) => {
        debug && console.log(identity, new Date().toTimeString(), 'REQUEST', req.method, req.originalUrl);
        res.on('finish', function () {
            if (this.statusCode !== 200 || debug) {
                console.log(new Date().toTimeString(), 'RESPONSE', req.method, req.originalUrl, this.statusCode);
            }
        })

        next();
    }
};
const extractProjectKeyFromRequest = function (req) {
    if (req.params.projectKey) {
        debug && console.log(`[WS]where projectKey=${req.params.projectKey}`);
        return req.params.projectKey;
    }
    return undefined;
}
const extractSessionIdFromRequest = function (req) {
    if (req.params.sessionId) {
        debug && console.log(`[WS]where sessionId=${req.params.sessionId}`);
        return req.params.sessionId;
    }
    return undefined;
}
const isValidSession = function (sessionInfo, filters) {
    let foundAll = true;
    for (const [key, values] of Object.entries(filters)) {
        let found = false;
        for (const [skey, svalue] of Object.entries(sessionInfo)) {
            if (svalue !== undefined && svalue !== null) {
                if (svalue.constructor === Object) {
                    if (isValidSession(svalue, {key: values})) {
                        found = true;
                        break;
                    }
                } else if (skey.toLowerCase() === key.toLowerCase()) {
                    for (let v of values) {
                        if (svalue.toLowerCase().indexOf(v.toLowerCase()) >= 0) {
                            found = true;
                            break;
                        }
                    }
                    if (found) {
                        break;
                    }
                }
            }
        }
        foundAll &&= found;
        if (!found) {
            break;
        }
    }
    return foundAll;
}
const getValidAttributes = function (sessionInfo, query) {
    let matches = [];
    let deduplicate = [];
    for (const [skey, svalue] of Object.entries(sessionInfo)) {
        if (svalue !== undefined && svalue !== null) {
            if (svalue.constructor === Object) {
                matches = [...matches, ...getValidAttributes(svalue, query)]
            } else if ((query.key === undefined || skey.toLowerCase() === query.key.toLowerCase())
                && svalue.toLowerCase().indexOf(query.value.toLowerCase()) >= 0
                && deduplicate.indexOf(skey + '_' + svalue) < 0) {
                matches.push({"type": skey, "value": svalue});
                deduplicate.push(skey + '_' + svalue);
            }
        }
    }
    return matches;
}
const hasFilters = function (filters) {
    return filters && filters.filter && Object.keys(filters.filter).length > 0;
}
const objectToObjectOfArrays = function (obj) {
    let _obj = {}
    for (let k of Object.keys(obj)) {
        if (obj[k] !== undefined && obj[k] !== null) {
            _obj[k] = obj[k];
            if (!Array.isArray(_obj[k])) {
                _obj[k] = [_obj[k]];
            }
            for (let i = 0; i < _obj[k].length; i++) {
                _obj[k][i] = String(_obj[k][i]);
            }
        }
    }
    return _obj;
}
const extractPayloadFromRequest = function (req) {
    let filters = {
        "query": {},
        "filter": {},
        "sort": {"key": undefined, "order": false},
        "pagination": {"limit": undefined, "page": undefined}
    };
    if (req.query.q) {
        debug && console.log(`[WS]where q=${req.query.q}`);
        filters.query.value = [req.query.q];
    }
    if (req.query.key) {
        debug && console.log(`[WS]where key=${req.query.key}`);
        filters.query.key = [req.query.key];
    }
    if (req.query.userId) {
        debug && console.log(`[WS]where userId=${req.query.userId}`);
        filters.filter.userID = [req.query.userId];
    }
    filters = objectToObjectOfArrays({...filters, ...(req.body.filter || {})});
    return filters;
}
const sortPaginate = function (list, filters) {
    list.sort((a, b) => {
        let aV = (a[filters.sort.key] || a["timestamp"]);
        let bV = (b[filters.sort.key] || b["timestamp"]);
        return aV > bV ? 1 : aV < bV ? -1 : 0;
    })

    if (filters.sort.order) {
        list.reverse();
    }

    if (filters.pagination.page && filters.pagination.limit) {
        return list.slice((filters.pagination.page - 1) * filters.pagination.limit,
            filters.pagination.page * filters.pagination.limit);
    }
    return list;
}
module.exports = {
    extractPeerId,
    request_logger,
    getValidAttributes,
    extractProjectKeyFromRequest,
    extractSessionIdFromRequest,
    isValidSession,
    hasFilters,
    objectToObjectOfArrays,
    extractPayloadFromRequest,
    sortPaginate
};