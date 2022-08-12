import React from 'react';
import { Button, PageTitle, Icon, Link } from 'UI';
import withPageTitle from 'HOCs/withPageTitle';
import MetricsList from '../MetricsList';
import MetricsSearch from '../MetricsSearch';
import { useStore } from 'App/mstore';
import { useObserver } from 'mobx-react-lite';

interface Props{
    siteId: number;
}
function MetricsView(props: Props) {
    const { siteId } = props;
    const { metricStore } = useStore();
    const metricsCount = useObserver(() => metricStore.metrics.length);

    React.useEffect(() => {
        metricStore.fetchList();
    }, []);
    return useObserver(() => (
        <div style={{ maxWidth: '1300px', margin: 'auto'}} className="bg-white rounded p-4">
            <div className="flex items-center mb-4 justify-between px-4">
                <div className="flex items-baseline mr-3">
                    <PageTitle title="Metrics" className="" />
                    <span className="text-2xl color-gray-medium ml-2">{metricsCount}</span>
                </div>
                <Link to={'/metrics/create'}><Button variant="primary">Create Metric</Button></Link>
                <div className="ml-auto w-1/4">
                    <MetricsSearch />
                </div>
            </div>
            <div className="text-xl text-disabled-text flex items-center pl-4">
                <Icon name="info-circle-fill" className="mr-2" size={18} />
                Create custom Metrics to capture key interactions and track KPIs.
            </div>
            <MetricsList />
        </div>
    ));
}

export default withPageTitle('Metrics - OpenReplay')(MetricsView);
