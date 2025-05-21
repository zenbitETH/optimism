// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { ITournamentFactory } from "interfaces/dispute/ITournamentFactory.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title IDisputeGameFactory
 * @notice Interface for the DisputeGameFactory contract
 */
interface IDisputeGameFactory {
    /**
     * @notice Creates a new dispute game
     * @param gameType The type of the game to create
     * @param rootClaim The root claim of the game
     * @param extraData Extra data for the game
     * @return The address of the newly created game
     */
    function create(
        uint8 gameType,
        bytes32 rootClaim,
        bytes calldata extraData
    ) external returns (address);

    /**
     * @notice Sets the implementation for a game type
     * @param gameType The game type
     * @param impl The implementation address
     */
    function setImplementation(uint8 gameType, address impl) external;
}

/**
 * @title DAVEDisputeGameFactory
 * @notice Extension of DisputeGameFactory to support DAVE tournament-based dispute games
 */
contract DAVEDisputeGameFactory is Ownable {
    /// @notice The type identifier for DAVE dispute games
    uint8 public constant DAVE_DISPUTE_GAME_TYPE = 3;

    /// @notice The DisputeGameFactory contract
    IDisputeGameFactory public disputeGameFactory;

    /// @notice The TournamentFactory contract
    ITournamentFactory public tournamentFactory;

    /**
     * @notice Constructor for the DAVEDisputeGameFactory contract
     * @param _disputeGameFactory The address of the DisputeGameFactory contract
     * @param _tournamentFactory The address of the TournamentFactory contract
     */
    constructor(address _disputeGameFactory, address _tournamentFactory) {
        disputeGameFactory = IDisputeGameFactory(_disputeGameFactory);
        tournamentFactory = ITournamentFactory(_tournamentFactory);
    }

    /**
     * @notice Registers this contract as the implementation for DAVE dispute games
     * @dev Can only be called by the owner
     */
    function register() external onlyOwner {
        // Register this contract as the implementation for DAVE dispute games
        disputeGameFactory.setImplementation(DAVE_DISPUTE_GAME_TYPE, address(this));
    }

    /**
     * @notice Sets the tournament factory
     * @param _tournamentFactory The address of the tournament factory
     * @dev Can only be called by the owner
     */
    function setTournamentFactory(address _tournamentFactory) external onlyOwner {
        tournamentFactory = ITournamentFactory(_tournamentFactory);
    }

    /**
     * @notice Creates a new DAVE tournament-based dispute game
     * @param rootClaim The root claim of the game
     * @param extraData Extra data for the game (l2BlockNumber, challengePeriod, bondSize)
     * @return The address of the newly created game
     */
    function createDAVEDisputeGame(
        bytes32 rootClaim,
        bytes calldata extraData
    ) external returns (address) {
        return disputeGameFactory.create(
            DAVE_DISPUTE_GAME_TYPE,
            rootClaim,
            extraData
        );
    }

    /**
     * @notice Creates a new tournament for dispute resolution
     * @param rootClaim The root claim being disputed
     * @param extraData Extra data for tournament creation
     * @return The address of the newly created tournament
     * @dev This function is called by the DisputeGameFactory when creating a DAVE dispute game
     */
    function create(
        bytes32 rootClaim,
        bytes calldata extraData
    ) external returns (address) {
        // Decode L2 block number from extra data
        (uint256 l2BlockNumber, bytes memory tournamentExtraData) = abi.decode(extraData, (uint256, bytes));

        // Create tournament
        return tournamentFactory.createTournament(
            rootClaim,
            l2BlockNumber,
            tournamentExtraData
        );
    }
}