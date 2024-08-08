// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import {Time} from "@openzeppelin/contracts/utils/types/Time.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {EnumerableMap} from "@openzeppelin/contracts/utils/structs/EnumerableMap.sol";

import {IRegistry} from "@symbiotic/interfaces/common/IRegistry.sol";
import {IEntity} from "@symbiotic/interfaces/common/IEntity.sol";
import {IVault} from "@symbiotic/interfaces/vault/IVault.sol";
import {IBaseDelegator} from "@symbiotic/interfaces/delegator/IBaseDelegator.sol";
import {IBaseSlasher} from "@symbiotic/interfaces/slasher/IBaseSlasher.sol";
import {IOptInService} from "@symbiotic/interfaces/service/IOptInService.sol";
import {IEntity} from "@symbiotic/interfaces/common/IEntity.sol";
import {ISlasher} from "@symbiotic/interfaces/slasher/ISlasher.sol";
import {IVetoSlasher} from "@symbiotic/interfaces/slasher/IVetoSlasher.sol";

import {SimpleKeyRegistry32} from "./SimpleKeyRegistry32.sol";
import {MapWithTimeData} from "./utils/MapWithTimeData.sol";

contract SimpleMiddleware is SimpleKeyRegistry32, Ownable {
    using EnumerableMap for EnumerableMap.AddressToUintMap;
    using MapWithTimeData for EnumerableMap.AddressToUintMap;

    address public immutable NETWORK;
    address public immutable OPERATOR_REGISTRY;
    address public immutable VAULT_REGISTRY;
    address public immutable OPERATOR_NET_OPTIN;
    address public immutable NET_VAULT_OPTIN;
    address public immutable OWNER;
    uint48 public immutable EPOCH_DURATION;
    uint48 public immutable SLASHING_WINDOW;
    uint48 public immutable START_TIME;

    uint48 private constant INSTANT_SLASHER_TYPE = 0;
    uint48 private constant VETO_SLASHER_TYPE = 1;

    EnumerableMap.AddressToUintMap internal operators;
    EnumerableMap.AddressToUintMap internal vaults;
    mapping(uint48 => uint256) public totalStakeCache;
    mapping(uint48 => mapping(address => uint256)) public operatorStakeCache;

    error NotOperator();
    error NotVault();

    error OperatorNotOptedIn();
    error OperatorNotRegistred();
    error OperarorGracePeriodNotPassed();
    error OperatorAlreadyRegistred();

    error VaultAlreadyRegistred();
    error VaultEpochTooShort();
    error VaultGracePeriodNotPassed();

    error TooOldEpoch();
    error InvalidEpoch();

    error SlashingWindowTooShort();
    error TooBigSlashAmount();
    error UnknownSlasherType();

    struct ValidatorData {
        uint256 stake;
        bytes32 key;
    }

    modifier updateStakeCache(uint48 epoch) {
        if (totalStakeCache[epoch] == 0) {
            calcAndCacheStakes(epoch);
        }
        _;
    }

    constructor(
        address _network,
        address _operatorRegistry,
        address _vaultRegistry,
        address _netVaultOptin,
        address _operatorNetOptin,
        address _owner,
        uint48 _epochDuration,
        uint48 _minSlashingWindow
    ) SimpleKeyRegistry32() Ownable(_owner) {
        START_TIME = Time.timestamp();
        EPOCH_DURATION = _epochDuration;
        NETWORK = _network;
        OWNER = _owner;
        OPERATOR_REGISTRY = _operatorRegistry;
        VAULT_REGISTRY = _vaultRegistry;
        OPERATOR_NET_OPTIN = _operatorNetOptin;
        NET_VAULT_OPTIN = _netVaultOptin;
        SLASHING_WINDOW = _minSlashingWindow;

        if (SLASHING_WINDOW < EPOCH_DURATION) {
            revert SlashingWindowTooShort();
        }
    }

    function getEpochStartTs(uint48 epoch) public view returns (uint48 timestamp) {
        return START_TIME + epoch * EPOCH_DURATION;
    }

    function getEpochAtTs(uint48 timestamp) public view returns (uint48 epoch) {
        return (timestamp - START_TIME) / EPOCH_DURATION;
    }

    function getCurrentEpoch() public view returns (uint48 epoch) {
        return getEpochAtTs(Time.timestamp());
    }

    function registerOperator(address operator, bytes32 key) external onlyOwner {
        if (operators.contains(operator)) {
            revert OperatorAlreadyRegistred();
        }

        if (!IRegistry(OPERATOR_REGISTRY).isEntity(operator)) {
            revert NotOperator();
        }

        if (!IOptInService(OPERATOR_NET_OPTIN).isOptedIn(operator, NETWORK)) {
            revert OperatorNotOptedIn();
        }

        updateKey(operator, key);

        operators.add(operator);
        operators.enable(operator);
    }

    function updateOperatorKey(address operator, bytes32 key) external onlyOwner {
        if (!operators.contains(operator)) {
            revert OperatorNotRegistred();
        }

        updateKey(operator, key);
    }

    function pauseOperator(address operator) external onlyOwner {
        operators.disable(operator);
    }

    function unpauseOperator(address operator) external onlyOwner {
        operators.enable(operator);
    }

    function unregisterOperator(address operator) external onlyOwner {
        (, uint48 disabledTime) = operators.getTimes(operator);

        if (disabledTime == 0 || disabledTime + SLASHING_WINDOW > Time.timestamp()) {
            revert OperarorGracePeriodNotPassed();
        }

        operators.remove(operator);
    }

    function registerVault(address vault) external onlyOwner {
        if (vaults.contains(vault)) {
            revert VaultAlreadyRegistred();
        }

        if (!IRegistry(VAULT_REGISTRY).isEntity(vault)) {
            revert NotVault();
        }

        uint48 vaultEpoch = IVault(vault).epochDuration();

        address slasher = IVault(vault).slasher();
        if (slasher != address(0) && IEntity(slasher).TYPE() == VETO_SLASHER_TYPE) {
            vaultEpoch -= IVetoSlasher(slasher).vetoDuration();
        }

        if (vaultEpoch < SLASHING_WINDOW) {
            revert VaultEpochTooShort();
        }

        vaults.add(vault);
        vaults.enable(vault);
    }

    function pauseVault(address vault) external onlyOwner {
        vaults.disable(vault);
    }

    function unpauseVault(address vault) external onlyOwner {
        vaults.enable(vault);
    }

    function unregisterVault(address vault) external onlyOwner {
        (, uint48 disabledTime) = vaults.getTimes(vault);

        if (disabledTime == 0 || disabledTime + SLASHING_WINDOW > Time.timestamp()) {
            revert VaultGracePeriodNotPassed();
        }

        vaults.remove(vault);
    }

    function getOperatorStake(address operator, uint48 epoch) public view returns (uint256 stake) {
        if (totalStakeCache[epoch] != 0) {
            return operatorStakeCache[epoch][operator];
        }

        uint48 epochStartTs = getEpochStartTs(epoch);

        for (uint256 i = 0; i < vaults.length(); ++i) {
            (address vault, uint48 enabledTime, uint48 disabledTime) = vaults.atWithTimes(i);

            // just skip the vault if it was enabled after the target epoch or not enabled
            if (!wasActiveAt(enabledTime, disabledTime, epochStartTs)) {
                continue;
            }

            stake += IBaseDelegator(IVault(vault).delegator()).stakeAt(NETWORK, operator, epochStartTs, new bytes(0));
        }

        return stake;
    }

    function getTotalStake(uint48 epoch) public view returns (uint256) {
        if (totalStakeCache[epoch] != 0) {
            return totalStakeCache[epoch];
        } else {
            return calcTotalStake(epoch);
        }
    }

    function getValidatorSet(uint48 epoch) public view returns (ValidatorData[] memory validatorsData) {
        uint48 epochStartTs = getEpochStartTs(epoch);

        validatorsData = new ValidatorData[](operators.length());
        uint256 valIdx = 0;

        for (uint256 i = 0; i < operators.length(); i++) {
            (address operator, uint48 enabledTime, uint48 disabledTime) = operators.atWithTimes(i);

            // just skip operator if it was added after the target epoch or paused
            if (!wasActiveAt(enabledTime, disabledTime, epochStartTs)) {
                continue;
            }

            bytes32 key = getOperatorKeyAt(operator, epochStartTs);
            if (key == bytes32(0)) {
                continue;
            }

            uint256 stake = getOperatorStake(operator, epochStartTs);

            validatorsData[valIdx++] = ValidatorData(stake, key);
        }

        // shrink array to skip unused slots
        /// @solidity memory-safe-assembly
        assembly {
            mstore(validatorsData, valIdx)
        }
    }

    function submission(bytes memory payload, bytes32[] memory signatures) public updateStakeCache(getCurrentEpoch()) {
        // validate signatures
        // validate payload
        // process payload
    }

    function slash(uint48 epoch, address operator, uint256 amount) public onlyOwner updateStakeCache(epoch) {
        uint48 epochStartTs = getEpochStartTs(epoch);

        if (epochStartTs < Time.timestamp() - SLASHING_WINDOW) {
            revert TooOldEpoch();
        }

        uint256 totalOperatorStake = getOperatorStake(operator, epoch);

        if (totalOperatorStake < amount) {
            revert TooBigSlashAmount();
        }

        // simple pro-rata slasher
        for (uint256 i = 0; i < vaults.length(); ++i) {
            (address vault, uint48 enabledTime, uint48 disabledTime) = operators.atWithTimes(i);

            // just skip the vault if it was enabled after the target epoch or not enabled
            if (!wasActiveAt(enabledTime, disabledTime, epochStartTs)) {
                continue;
            }

            uint256 vaultStake =
                IBaseDelegator(IVault(vault).delegator()).stakeAt(NETWORK, operator, epochStartTs, new bytes(0));

            slashVault(epochStartTs, vault, operator, amount * vaultStake / totalOperatorStake);
        }
    }

    function calcAndCacheStakes(uint48 epoch) public returns (uint256 totalStake) {
        uint48 epochStartTs = getEpochStartTs(epoch);

        // for epoch older than SLASHING_WINDOW total stake can be invalidated (use cache)
        if (epochStartTs < Time.timestamp() - SLASHING_WINDOW) {
            revert TooOldEpoch();
        }

        if (epochStartTs > Time.timestamp()) {
            revert InvalidEpoch();
        }

        for (uint256 i = 0; i < operators.length(); i++) {
            (address operator, uint48 enabledTime, uint48 disabledTime) = operators.atWithTimes(i);

            // just skip operator if it was added after the target epoch or paused
            if (!wasActiveAt(enabledTime, disabledTime, epochStartTs)) {
                continue;
            }

            uint256 operatorStake = getOperatorStake(operator, epochStartTs);
            operatorStakeCache[epoch][operator] = operatorStake;

            totalStake += operatorStake;
        }

        if (totalStake == 0) {
            // just keep 1 to avoid cache recalculations
            totalStakeCache[epoch] = 1;
        } else {
            totalStakeCache[epoch] = totalStake;
        }
    }

    function calcTotalStake(uint48 epoch) internal view returns (uint256 totalStake) {
        uint48 epochStartTs = getEpochStartTs(epoch);

        // for epoch older than SLASHING_WINDOW total stake can be invalidated (use cache)
        if (epochStartTs < Time.timestamp() - SLASHING_WINDOW) {
            revert TooOldEpoch();
        }

        if (epochStartTs > Time.timestamp()) {
            revert InvalidEpoch();
        }

        for (uint256 i = 0; i < operators.length(); i++) {
            (address operator, uint48 enabledTime, uint48 disabledTime) = operators.atWithTimes(i);

            // just skip operator if it was added after the target epoch or paused
            if (!wasActiveAt(enabledTime, disabledTime, epochStartTs)) {
                continue;
            }

            uint256 operatorStake = getOperatorStake(operator, epochStartTs);
            totalStake += operatorStake;
        }
    }

    function wasActiveAt(uint48 enabledTime, uint48 disabledTime, uint48 timestamp) internal pure returns (bool) {
        if (enabledTime > timestamp || (disabledTime != 0 && disabledTime < timestamp)) {
            return false;
        }
        return true;
    }

    function slashVault(uint48 timestamp, address vault, address operator, uint256 amount) internal {
        address slasher = IVault(vault).slasher();
        uint256 slasherType = IEntity(slasher).TYPE();
        if (slasherType == INSTANT_SLASHER_TYPE) {
            ISlasher(slasher).slash(NETWORK, operator, amount, timestamp, new bytes(0));
        } else if (slasherType == VETO_SLASHER_TYPE) {
            IVetoSlasher(slasher).requestSlash(NETWORK, operator, amount, timestamp, new bytes(0));
        } else {
            revert UnknownSlasherType();
        }
    }
}
