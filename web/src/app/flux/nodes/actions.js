import reactor from 'app/reactor';
import { TLPT_NODES_RECEIVE } from './actionTypes';
import api from 'app/services/api';
import cfg from 'app/config';
import appGetters from 'app/flux/app/getters';
import Logger from 'app/lib/logger';
const logger = Logger.create('Modules/Nodes');

export function fetchNodes() {
  console.log("called fetchNodes");
  console.log("============");
  console.log("called fetchNodes");
  
  const siteId = reactor.evaluate(appGetters.siteId);
  return api.get(cfg.api.getSiteNodesUrl(siteId))
    .then(res => {
      console.log("called fetchNodes");
      console.log(res.items);
      console.log("called fetchNodes");
      return res.items || []
    })
    .done(items => reactor.dispatch(TLPT_NODES_RECEIVE, items))
    .fail(err => logger.error('fetchNodes', err));
}