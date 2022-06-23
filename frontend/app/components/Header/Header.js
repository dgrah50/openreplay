import React, { useEffect, useState } from 'react';
import { connect } from 'react-redux';
import { NavLink, withRouter } from 'react-router-dom';
import cn from 'classnames';
import { 
  sessions,
  assist,
  client,
  errors,
  // funnels,
  dashboard,
  withSiteId,
  CLIENT_DEFAULT_TAB,
  isRoute,
} from 'App/routes';
import { logout } from 'Duck/user';
import { Icon, Popup } from 'UI';
import SiteDropdown from './SiteDropdown';
import styles from './header.module.css';
import OnboardingExplore from './OnboardingExplore/OnboardingExplore'
import Announcements from '../Announcements';
import Notifications from '../Alerts/Notifications';
import { init as initSite, fetchList as fetchSiteList } from 'Duck/site';

import ErrorGenPanel from 'App/dev/components';
import Alerts from '../Alerts/Alerts';
import AnimatedSVG, { ICONS } from '../shared/AnimatedSVG/AnimatedSVG';
import { fetchList as fetchMetadata } from 'Duck/customField';
import { useStore } from 'App/mstore';

const DASHBOARD_PATH = dashboard();
const SESSIONS_PATH = sessions();
const ASSIST_PATH = assist();
const ERRORS_PATH = errors();
// const FUNNELS_PATH = funnels();
const CLIENT_PATH = client(CLIENT_DEFAULT_TAB);
const AUTOREFRESH_INTERVAL = 30 * 1000;

let interval = null;

const Header = (props) => {
  const { 
    sites, location, account, 
    onLogoutClick, siteId,
    boardingCompletion = 100, fetchSiteList, showAlerts = false
  } = props;
  
  const name = account.get('name').split(" ")[0];
  const [hideDiscover, setHideDiscover] = useState(false)
  const { userStore } = useStore();
  let activeSite = null;

  useEffect(() => {
    userStore.fetchLimits();
  }, []);

  useEffect(() => {
    activeSite = sites.find(s => s.id == siteId);
    props.initSite(activeSite);
    props.fetchMetadata();
  }, [sites])

  return (
    <div className={ cn(styles.header) }>
      <NavLink to={ withSiteId(SESSIONS_PATH, siteId) }>
        <div className="relative">
          <div className="p-2">
            <AnimatedSVG name={ICONS.LOGO_SMALL} size="30" />
          </div>
          <div className="absolute bottom-0" style={{ fontSize: '7px', right: '5px' }}>v{window.env.VERSION}</div>
        </div>
      </NavLink>
      <SiteDropdown />
      <div className={ styles.divider } />

      <NavLink
        to={ withSiteId(SESSIONS_PATH, siteId) }
        className={ styles.nav }
        activeClassName={ styles.active }
      >
        { 'Sessions' }
      </NavLink>
      <NavLink
        to={ withSiteId(ASSIST_PATH, siteId) }
        className={ styles.nav }
        activeClassName={ styles.active }
      >
        { 'Assist' }
      </NavLink>
      {/* <NavLink
        to={ withSiteId(ERRORS_PATH, siteId) }
        className={ styles.nav }
        activeClassName={ styles.active }
      >
        { 'Errors' }
      </NavLink>
      <NavLink
        to={ withSiteId(FUNNELS_PATH, siteId) }
        className={ styles.nav }
        activeClassName={ styles.active }
      >
        { 'Funnels' }
      </NavLink> */}
      <NavLink
        to={ withSiteId(DASHBOARD_PATH, siteId) }
        className={ styles.nav }
        activeClassName={ styles.active }
      >         
        <span>{ 'Dashboards' }</span>
      </NavLink>
      <div className={ styles.right }>
        <Announcements />
        <div className={ styles.divider } />

        { (boardingCompletion < 100 && !hideDiscover) && (
          <React.Fragment>            
            <OnboardingExplore onComplete={() => setHideDiscover(true)} />
            <div className={ styles.divider } />
          </React.Fragment>
        )}
      
        <Notifications />
        <div className={ styles.divider } />
        <Popup content={ `Preferences` } >
          <NavLink to={ CLIENT_PATH } className={ styles.headerIcon }><Icon name="cog" size="20" /></NavLink>
        </Popup>
        
        <div className={ styles.divider } />
        <div className={ styles.userDetails }>
          <div className="flex items-center">
            <div className="mr-5">{ name }</div>
            <Icon color="gray-medium" name="ellipsis-v" size="24" />
          </div>

          <ul>
            <li><button onClick={ onLogoutClick }>{ 'Logout' }</button></li>
          </ul>
        </div>
      </div>
      { <ErrorGenPanel/> }
      {showAlerts && <Alerts />}
    </div>
  );
};

export default withRouter(connect(
  state => ({
    account: state.getIn([ 'user', 'account' ]),
    siteId: state.getIn([ 'site', 'siteId' ]),
    sites: state.getIn([ 'site', 'list' ]),
    showAlerts: state.getIn([ 'dashboard', 'showAlerts' ]),
    boardingCompletion: state.getIn([ 'dashboard', 'boardingCompletion' ])
  }),
  { onLogoutClick: logout, initSite, fetchSiteList, fetchMetadata },
)(Header));
