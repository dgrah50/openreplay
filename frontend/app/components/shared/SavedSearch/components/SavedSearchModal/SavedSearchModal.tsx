import React, { MouseEvent, useState } from 'react'
import cn from 'classnames';
import { Icon as SemIcon } from 'semantic-ui-react';
import { Icon, Input } from 'UI';
import { List } from 'immutable';
import { Tooltip } from 'react-tippy'
import { confirm } from 'UI/Confirmation';
import { applySavedSearch, remove, editSavedSearch } from 'Duck/search'
import { connect } from 'react-redux';
import { useModal } from 'App/components/Modal';
import { SavedSearch } from 'Types/ts/search'
import SaveSearchModal from 'Shared/SaveSearchModal'
import stl from './savedSearchModal.css'


interface ITooltipIcon {
    title: string;
    name: string;
    onClick: (e: MouseEvent<HTMLDivElement>) => void;
}
function TooltipIcon(props: ITooltipIcon) {
    return (
        <div onClick={(e) => props.onClick(e)} >
            {/* @ts-ignore - problem with react-tippy types TODO: remove after fix */}
            <Tooltip
                title={props.title}
                hideOnClick={true}
                position="bottom"
            >
                <Icon size="16" name={props.name} color="main" />
            </Tooltip>
        </div>
    )
}

interface Props {
    list: List<SavedSearch>;
    applySavedSearch: (item: SavedSearch) => void;
    remove: (itemId: number) => void;
    editSavedSearch: (item: SavedSearch) => void;
}
function SavedSearchModal(props: Props) {
    const { hideModal } = useModal();
    const [showModal, setshowModal] = useState(false)
    const [filterQuery, setFilterQuery] = useState('')

    const onClick = (item: SavedSearch, e) => {
        e.stopPropagation();
        props.applySavedSearch(item);
        hideModal();
    }
    const onDelete = async (item: SavedSearch, e: MouseEvent<HTMLDivElement>) => {
        e.stopPropagation();
        const confirmation = await confirm({
            header: 'Confirm',
            confirmButton: 'Yes, delete',
            confirmation: 'Are you sure you want to permanently delete this search?'
        })
        if (confirmation) {
            props.remove(item.searchId)
        }
    }
    const onEdit = (item: SavedSearch, e: MouseEvent<HTMLDivElement>) => {
        e.stopPropagation();
        props.editSavedSearch(item);
        setTimeout(() => setshowModal(true), 0);
    }

    const shownItems = props.list.filter(item => item.name.includes(filterQuery))

    return (
        <div className="bg-white box-shadow h-screen" style={{ width: '450px' }}>
            <div className="p-6">
                <h1 className="text-2xl">Saved Search  <span className="color-gray-medium">{props.list.size}</span></h1>
            </div>
            {props.list.size > 1 && (
                <div className="mb-6 w-full px-4">
                    <Input
                        className="w-full"
                        iconPosition="left"
                        icon={<SemIcon name="search" />}
                        onChange={(_, v) => setFilterQuery(v.value)}
                        placeholder="Filter by name"
                    />
                </div>
            )}
            {shownItems.map(item => (
                <div key={item.key} className="p-4 pb-8 cursor-pointer border-b flex items-center group hover:bg-active-blue" onClick={(e) => onClick(item, e)}>
                    <Icon name="search" color="gray-medium" size="16" />
                    <div className="ml-4">
                        <div className="text-lg">{item.name} </div>
                        {item.isPublic && (
                            <div className={cn(stl.iconContainer, 'absolute color-gray-medium flex items-center px-2 mt-2')}>
                                <Icon name="user-friends" size="11" />
                                <div className="ml-1 text-sm"> Team </div>
                            </div>
                        )}
                    </div>
                    <div className="flex items-center ml-auto self-center">
                        <div className={cn(stl.iconCircle, 'mr-2 invisible group-hover:visible')}>
                            <TooltipIcon name="pencil" onClick={(e) => onEdit(item, e)} title="Rename" />
                        </div>
                        <div className={cn(stl.iconCircle, 'invisible group-hover:visible')}>
                            <TooltipIcon name="trash" onClick={(e) => onDelete(item, e)} title="Delete" />
                        </div>
                    </div>
                </div>
            ))}
             { showModal && ( <SaveSearchModal show closeHandler={() => setshowModal(false)} /> )}
        </div>
    )
}

export default React.memo(connect(null, { applySavedSearch, remove, editSavedSearch })(SavedSearchModal))
