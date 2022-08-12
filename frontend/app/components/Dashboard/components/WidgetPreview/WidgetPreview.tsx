import React from 'react';
import cn from 'classnames';
import WidgetWrapper from '../WidgetWrapper';
import { useStore } from 'App/mstore';
import { SegmentSelection, Button, Icon } from 'UI';
import { useObserver } from 'mobx-react-lite';
import SelectDateRange from 'Shared/SelectDateRange';
import { FilterKey } from 'Types/filter/filterType';
import WidgetDateRange from '../WidgetDateRange/WidgetDateRange';
// import Period, { LAST_24_HOURS, LAST_30_DAYS } from 'Types/app/period';
import DashboardSelectionModal from '../DashboardSelectionModal/DashboardSelectionModal';

interface Props {
    className?: string;
}
function WidgetPreview(props: Props) {
    const [showDashboardSelectionModal, setShowDashboardSelectionModal] = React.useState(false);
    const { className = '' } = props;
    const { metricStore, dashboardStore } = useStore();
    const dashboards = dashboardStore.dashboards;
    const metric: any = useObserver(() => metricStore.instance);
    const isTimeSeries = metric.metricType === 'timeseries';
    const isTable = metric.metricType === 'table';
    const drillDownFilter = useObserver(() => dashboardStore.drillDownFilter);
    const disableVisualization = useObserver(() => metric.metricOf === FilterKey.SESSIONS || metric.metricOf === FilterKey.ERRORS);
    // const period = useObserver(() => dashboardStore.drillDownPeriod);

    const chagneViewType = (e, { name, value }: any) => {
        metric.update({ [ name ]: value });
    }

    // const onChangePeriod = (period: any) => {
    //     dashboardStore.setDrillDownPeriod(period);
    //     const periodTimestamps = period.toTimestamps();
    //     drillDownFilter.merge({
    //         startTimestamp: periodTimestamps.startTimestamp,
    //         endTimestamp: periodTimestamps.endTimestamp,
    //     })
    // }

    const getWidgetTitle = () => {
        if (isTimeSeries) {
            return 'Time Series';
        } else if (isTable) {
            if (metric.metricOf === FilterKey.SESSIONS) {
                // return 'Table of Sessions';
                return <div>Sessions <span className="color-gray-medium">{metric.data.total}</span></div>;
            } else if (metric.metricOf === FilterKey.ERRORS) {
                // return 'Table of Errors';
                return <div>Errors <span className="color-gray-medium">{metric.data.total}</span></div>;
            } else {
                return 'Table';
            }
        } else if (metric.metricType === 'funnel') {
            return 'Funnel';
        }
    }

    const canAddToDashboard = metric.exists() && dashboards.length > 0;

    return useObserver(() => (
        <>
        <div className={cn(className, 'bg-white rounded')}>
            <div className="flex items-center justify-between px-4 pt-2">
                <h2 className="text-2xl">
                    {getWidgetTitle()}
                </h2>
                <div className="flex items-center">
                    {isTimeSeries && (
                        <>
                            <span className="mr-4 color-gray-medium">Visualization</span>
                            <SegmentSelection
                                name="viewType"
                                className="my-3"
                                primary
                                icons={true}
                                onSelect={ chagneViewType }
                                value={{ value: metric.viewType }}
                                list={ [
                                    { value: 'lineChart', name: 'Chart', icon: 'graph-up-arrow' },
                                    { value: 'progress', name: 'Progress', icon: 'hash' },
                                ]}
                            />
                        </>
                    )}

                    {isTable && (
                        <>
                            <span className="mr-4 color-gray-medium">Visualization</span>
                            <SegmentSelection
                                name="viewType"
                                className="my-3"
                                primary={true}
                                icons={true}
                                onSelect={ chagneViewType }
                                value={{ value: metric.viewType }}
                                list={[
                                    { value: 'table', name: 'Table', icon: 'table' },
                                    { value: 'pieChart', name: 'Chart', icon: 'pie-chart-fill' },
                                ]}
                                disabled={disableVisualization}
                                disabledMessage="Chart view is not supported"
                            />
                        </>
                    )}
                    <div className="mx-4" />
                    <WidgetDateRange />
                    {/* add to dashboard */}
                    {metric.exists() && (
                        <Button
                        variant="text-primary"
                        className="ml-2 p-0"
                        onClick={() => setShowDashboardSelectionModal(true)}
                        disabled={!canAddToDashboard}
                    > 
                        <Icon name="columns-gap-filled" size="14" className="mr-2" color="teal"/>
                        Add to Dashboard
                    </Button>
                    )}
                </div>
            </div>
            <div className="p-4 pt-0">
                <WidgetWrapper widget={metric} isPreview={true} isWidget={false} />
            </div>
        </div>
        { canAddToDashboard && (
            <DashboardSelectionModal
                metricId={metric.metricId}
                show={showDashboardSelectionModal}
                closeHandler={() => setShowDashboardSelectionModal(false)}
            />
        )}
        </>
    ));
}

export default WidgetPreview;
